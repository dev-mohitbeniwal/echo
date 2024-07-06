package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/dev-mohitbeniwal/echo/api/dao"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

// PolicyService handles business logic for policy operations
type PolicyService struct {
	policyDAO       *dao.PolicyDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var ErrPolicyNotFound = errors.New("policy not found")

// NewPolicyService creates a new instance of PolicyService
func NewPolicyService(policyDAO *dao.PolicyDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *PolicyService {
	service := &PolicyService{
		policyDAO:       policyDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("policy.created", service.handlePolicyCreated)
	eventBus.Subscribe("policy.updated", service.handlePolicyUpdated)
	eventBus.Subscribe("policy.deleted", service.handlePolicyDeleted)

	return service
}

func (s *PolicyService) handlePolicyCreated(ctx context.Context, event util.Event) error {
	policy := event.Payload.(model.Policy)
	logger.Info("Policy created event received", zap.String("policyID", policy.ID))

	// Update any indexes or materialized views
	if err := s.updatePolicyIndexes(ctx, policy); err != nil {
		logger.Error("Failed to update policy indexes", zap.Error(err), zap.String("policyID", policy.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPolicyChange(ctx, "created", policy); err != nil {
		logger.Warn("Failed to send policy creation notification", zap.Error(err), zap.String("policyID", policy.ID))
	}

	// Trigger any necessary recomputations or cache invalidations
	if err := s.triggerPolicyDependentUpdates(ctx, policy.ID); err != nil {
		logger.Error("Failed to trigger policy-dependent updates", zap.Error(err), zap.String("policyID", policy.ID))
		return err
	}

	return nil
}

func (s *PolicyService) handlePolicyUpdated(ctx context.Context, event util.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		logger.Error("Invalid event payload type", zap.Any("payload", event.Payload))
		return fmt.Errorf("invalid event payload type: %T", event.Payload)
	}

	var oldPolicy, newPolicy model.Policy
	if old, ok := payload["old"]; ok {
		oldPolicy, ok = old.(model.Policy)
		if !ok {
			logger.Error("Invalid old policy type", zap.Any("oldPolicy", old))
			return fmt.Errorf("invalid old policy type: %T", old)
		}
	} else {
		logger.Error("Old policy not found in event payload", zap.Any("payload", payload))
		return errors.New("old policy not found in event payload")
	}

	if new, ok := payload["new"]; ok {
		newPolicy, ok = new.(model.Policy)
		if !ok {
			logger.Error("Invalid new policy type", zap.Any("newPolicy", new))
			return fmt.Errorf("invalid new policy type: %T", new)
		}
	} else {
		logger.Error("New policy not found in event payload", zap.Any("payload", payload))
		return errors.New("new policy not found in event payload")
	}

	logger.Info("Policy updated event received",
		zap.String("policyID", newPolicy.ID),
		zap.Int("oldVersion", oldPolicy.Version),
		zap.Int("newVersion", newPolicy.Version))

	// Update any indexes or materialized views
	if err := s.updatePolicyIndexes(ctx, newPolicy); err != nil {
		logger.Error("Failed to update policy indexes", zap.Error(err), zap.String("policyID", newPolicy.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPolicyChange(ctx, "updated", newPolicy); err != nil {
		logger.Warn("Failed to send policy update notification", zap.Error(err), zap.String("policyID", newPolicy.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this policy change
	if err := s.invalidateRelatedCaches(ctx, newPolicy.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("policyID", newPolicy.ID))
		// Continue execution despite the error
	}

	// Recompute access decisions that might be affected by this policy change
	if err := s.recomputeAffectedAccessDecisions(ctx, oldPolicy, newPolicy); err != nil {
		logger.Error("Failed to recompute affected access decisions", zap.Error(err), zap.String("policyID", newPolicy.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *PolicyService) handlePolicyDeleted(ctx context.Context, event util.Event) error {
	policyID, ok := event.Payload.(string)
	if !ok {
		logger.Error("Invalid event payload type", zap.Any("payload", event.Payload))
		return fmt.Errorf("invalid event payload type: %T", event.Payload)
	}

	logger.Info("Policy deleted event received", zap.String("policyID", policyID))

	// Remove policy from any indexes or materialized views
	if err := s.removePolicyFromIndexes(ctx, policyID); err != nil {
		logger.Error("Failed to remove policy from indexes", zap.Error(err), zap.String("policyID", policyID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPolicyChange(ctx, "deleted", model.Policy{ID: policyID}); err != nil {
		logger.Warn("Failed to send policy deletion notification", zap.Error(err), zap.String("policyID", policyID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupPolicyRelatedData(ctx, policyID); err != nil {
		logger.Error("Failed to clean up policy-related data", zap.Error(err), zap.String("policyID", policyID))
		// Continue execution despite the error
	}

	// Recompute access decisions that might be affected by this policy deletion
	if err := s.recomputeAffectedAccessDecisions(ctx, model.Policy{ID: policyID}, model.Policy{}); err != nil {
		logger.Error("Failed to recompute affected access decisions", zap.Error(err), zap.String("policyID", policyID))
		// Continue execution despite the error
	}

	return nil
}

// CreatePolicy handles the creation of a new policy
func (s *PolicyService) CreatePolicy(ctx context.Context, policy model.Policy, userID string) (*model.Policy, error) {
	if err := s.validationUtil.ValidatePolicy(policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	if err := s.checkPolicyConflicts(ctx, policy); err != nil {
		return nil, fmt.Errorf("policy conflict: %w", err)
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Version = 1

	policyID, err := s.policyDAO.CreatePolicy(ctx, policy, userID)
	if err != nil {
		logger.Error("Error creating policy", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	policy.ID = policyID

	// Update cache
	if err := s.cacheService.SetPolicy(ctx, policy); err != nil {
		logger.Warn("Failed to cache policy", zap.Error(err), zap.String("policyID", policyID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "policy.created", policy)

	logger.Info("Policy created successfully", zap.String("policyID", policyID), zap.String("userID", userID))
	return &policy, nil
}

// UpdatePolicy handles updates to an existing policy
func (s *PolicyService) UpdatePolicy(ctx context.Context, policy model.Policy, userID string) (*model.Policy, error) {
	if err := s.validationUtil.ValidatePolicy(policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	if err := s.checkPolicyConflicts(ctx, policy); err != nil {
		return nil, fmt.Errorf("policy conflict: %w", err)
	}

	oldPolicy, err := s.policyDAO.GetPolicy(ctx, policy.ID)
	if err != nil {
		logger.Error("Error retrieving existing policy", zap.Error(err), zap.String("policyID", policy.ID))
		return nil, err
	}

	// Check if there are any differences between the old and new policies
	if !s.hasPolicyChanged(oldPolicy, &policy) {
		logger.Info("No changes detected in the policy, skipping update", zap.String("policyID", policy.ID))
		return oldPolicy, nil
	}

	policy.UpdatedAt = time.Now()
	policy.Version = oldPolicy.Version + 1

	updatedPolicy, err := s.policyDAO.UpdatePolicy(ctx, policy, userID)
	if err != nil {
		logger.Error("Error updating policy", zap.Error(err), zap.String("policyID", policy.ID), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetPolicy(ctx, *updatedPolicy); err != nil {
		logger.Warn("Failed to update policy in cache", zap.Error(err), zap.String("policyID", policy.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "policy.updated", map[string]interface{}{
		"old": *oldPolicy,
		"new": *updatedPolicy,
	})

	logger.Info("Policy updated successfully", zap.String("policyID", policy.ID), zap.String("userID", userID))
	return updatedPolicy, nil
}

// DeletePolicy handles the deletion of a policy
func (s *PolicyService) DeletePolicy(ctx context.Context, policyID string, userID string) error {
	err := s.policyDAO.DeletePolicy(ctx, policyID, userID)
	if err != nil {
		logger.Error("Error deleting policy", zap.Error(err), zap.String("policyID", policyID), zap.String("userID", userID))
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeletePolicy(ctx, policyID); err != nil {
		logger.Warn("Failed to delete policy from cache", zap.Error(err), zap.String("policyID", policyID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "policy.deleted", policyID)

	logger.Info("Policy deleted successfully", zap.String("policyID", policyID), zap.String("userID", userID))
	return nil
}

// GetPolicy retrieves a policy by its ID
func (s *PolicyService) GetPolicy(ctx context.Context, policyID string) (*model.Policy, error) {
	// Try to get from cache first
	cachedPolicy, err := s.cacheService.GetPolicy(ctx, policyID)
	if err == nil && cachedPolicy != nil {
		return cachedPolicy, nil
	}

	policy, err := s.policyDAO.GetPolicy(ctx, policyID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrPolicyNotFound) {
			return nil, echo_errors.ErrPolicyNotFound
		}
		logger.Error("Error retrieving policy", zap.Error(err), zap.String("policyID", policyID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetPolicy(ctx, *policy); err != nil {
		logger.Warn("Failed to cache policy", zap.Error(err), zap.String("policyID", policyID))
	}

	return policy, nil
}

// ListPolicies retrieves all policies, possibly with pagination
func (s *PolicyService) ListPolicies(ctx context.Context, limit int, offset int) ([]*model.Policy, error) {
	policies, err := s.policyDAO.ListPolicies(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing policies", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}

	return policies, nil
}

// BulkCreatePolicies creates multiple policies in parallel
func (s *PolicyService) BulkCreatePolicies(ctx context.Context, policies []model.Policy, userID string) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)
	policyIDs := make([]string, len(policies))

	// Limit concurrency to avoid overwhelming the system
	semaphore := make(chan struct{}, 10) // Adjust this number based on your system's capacity

	for i, policy := range policies {
		i, policy := i, policy
		g.Go(func() error {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			createdPolicy, err := s.CreatePolicy(ctx, policy, userID)
			if err != nil {
				return err
			}
			policyIDs[i] = createdPolicy.ID
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		logger.Error("Error in bulk create policies", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to bulk create policies: %w", err)
	}

	logger.Info("Bulk create policies completed", zap.Int("count", len(policyIDs)), zap.String("userID", userID))
	return policyIDs, nil
}

// SearchPolicies searches for policies based on given criteria
func (s *PolicyService) SearchPolicies(ctx context.Context, criteria model.PolicySearchCriteria) ([]*model.Policy, error) {
	policies, err := s.policyDAO.SearchPolicies(ctx, criteria)
	if err != nil {
		logger.Error("Error searching policies", zap.Error(err), zap.Any("criteria", criteria))
		return nil, fmt.Errorf("failed to search policies: %w", err)
	}

	return policies, nil
}

// AnalyzePolicyUsage analyzes the usage of a policy
func (s *PolicyService) AnalyzePolicyUsage(ctx context.Context, policyID string) (*model.PolicyUsageAnalysis, error) {
	analysis, err := s.policyDAO.AnalyzePolicyUsage(ctx, policyID)
	if err != nil {
		logger.Error("Error analyzing policy usage", zap.Error(err), zap.String("policyID", policyID))
		return nil, fmt.Errorf("failed to analyze policy usage: %w", err)
	}

	return analysis, nil
}

// checkPolicyConflicts checks if the given policy conflicts with existing policies
func (s *PolicyService) checkPolicyConflicts(ctx context.Context, policy model.Policy) error {
	// Implement logic to check for conflicts with existing policies
	// This could involve checking for overlapping subjects, resources, or actions
	// Return an error if a conflict is found
	return nil
}

// hasPolicyChanged checks if there are any differences between the old and new policies
func (s *PolicyService) hasPolicyChanged(oldPolicy, newPolicy *model.Policy) bool {
	if oldPolicy.Name != newPolicy.Name ||
		oldPolicy.Description != newPolicy.Description ||
		oldPolicy.Effect != newPolicy.Effect ||
		oldPolicy.Priority != newPolicy.Priority ||
		oldPolicy.Active != newPolicy.Active ||
		!reflect.DeepEqual(oldPolicy.Subjects, newPolicy.Subjects) ||
		!reflect.DeepEqual(oldPolicy.Resources, newPolicy.Resources) ||
		!reflect.DeepEqual(oldPolicy.Actions, newPolicy.Actions) ||
		!reflect.DeepEqual(oldPolicy.Conditions, newPolicy.Conditions) {
		return true
	}
	return false
}

// updatePolicyIndexes updates any search indexes or materialized views for quick policy lookups
func (s *PolicyService) updatePolicyIndexes(ctx context.Context, policy model.Policy) error {
	logger.Info("Updating policy indexes", zap.String("policyID", policy.ID))

	// This is a placeholder for actual index update logic
	// In a real implementation, you might:
	// 1. Update a search index (e.g., Elasticsearch)
	// 2. Update a materialized view in your database
	// 3. Update any caching layers

	// Example: Update a search index
	/*
	   indexDoc := map[string]interface{}{
	       "id":          policy.ID,
	       "name":        policy.Name,
	       "description": policy.Description,
	       "effect":      policy.Effect,
	       "subjects":    policy.Subjects,
	       "resources":   policy.Resources,
	       "actions":     policy.Actions,
	       "conditions":  policy.Conditions,
	       "updated_at":  policy.UpdatedAt,
	   }
	   _, err := s.searchClient.Index().
	       Index("policies").
	       Id(policy.ID).
	       BodyJson(indexDoc).
	       Do(ctx)
	   if err != nil {
	       return fmt.Errorf("failed to update search index: %w", err)
	   }
	*/

	return nil
}

// triggerPolicyDependentUpdates initiates processes that depend on policy changes
func (s *PolicyService) triggerPolicyDependentUpdates(ctx context.Context, policyID string) error {
	logger.Info("Triggering policy-dependent updates", zap.String("policyID", policyID))

	// This is a placeholder for actual update logic
	// In a real implementation, you might:
	// 1. Re-evaluate access rights for affected users
	// 2. Update any derived data structures
	// 3. Trigger updates in dependent systems

	// Example: Re-evaluate access rights
	/*
	   affectedUsers, err := s.findAffectedUsers(ctx, policyID)
	   if err != nil {
	       return fmt.Errorf("failed to find affected users: %w", err)
	   }
	   for _, userID := range affectedUsers {
	       if err := s.reevaluateUserAccess(ctx, userID); err != nil {
	           logger.Error("Failed to re-evaluate user access", zap.Error(err), zap.String("userID", userID))
	       }
	   }
	*/

	return nil
}

// invalidateRelatedCaches clears any caches that might contain outdated information due to the policy change
func (s *PolicyService) invalidateRelatedCaches(ctx context.Context, policyID string) error {
	logger.Info("Invalidating related caches", zap.String("policyID", policyID))

	// This is a placeholder for actual cache invalidation logic
	// In a real implementation, you might:
	// 1. Clear specific cache entries related to this policy
	// 2. Clear user permission caches
	// 3. Notify other services to clear their caches

	// Example: Clear cache entries
	/*
	   cacheKeys := []string{
	       fmt.Sprintf("policy:%s", policyID),
	       "all_policies",
	       "policy_list",
	   }
	   for _, key := range cacheKeys {
	       if err := s.cacheService.Delete(ctx, key); err != nil {
	           logger.Warn("Failed to delete cache key", zap.Error(err), zap.String("key", key))
	       }
	   }
	*/

	return nil
}

// recomputeAffectedAccessDecisions re-evaluates access decisions that were based on the changed or deleted policy
func (s *PolicyService) recomputeAffectedAccessDecisions(ctx context.Context, oldPolicy, newPolicy model.Policy) error {
	logger.Info("Recomputing affected access decisions",
		zap.String("policyID", oldPolicy.ID),
		zap.Bool("isDelete", newPolicy.ID == ""))

	// This is a placeholder for actual recomputation logic
	// In a real implementation, you might:
	// 1. Identify all access decisions that were influenced by this policy
	// 2. Re-evaluate each of these decisions
	// 3. Update any persistent stores of access decisions

	// Example: Re-evaluate decisions
	/*
	   affectedDecisions, err := s.findAffectedDecisions(ctx, oldPolicy.ID)
	   if err != nil {
	       return fmt.Errorf("failed to find affected decisions: %w", err)
	   }
	   for _, decision := range affectedDecisions {
	       newDecision, err := s.evaluateAccess(ctx, decision.UserID, decision.ResourceID, decision.Action)
	       if err != nil {
	           logger.Error("Failed to re-evaluate access", zap.Error(err), zap.Any("decision", decision))
	           continue
	       }
	       if err := s.updateAccessDecision(ctx, newDecision); err != nil {
	           logger.Error("Failed to update access decision", zap.Error(err), zap.Any("decision", newDecision))
	       }
	   }
	*/

	return nil
}

// removePolicyFromIndexes removes the policy from any indexes or views when it's deleted
func (s *PolicyService) removePolicyFromIndexes(ctx context.Context, policyID string) error {
	logger.Info("Removing policy from indexes", zap.String("policyID", policyID))

	// This is a placeholder for actual index removal logic
	// In a real implementation, you might:
	// 1. Remove the policy from search indexes
	// 2. Update materialized views
	// 3. Remove any denormalized data related to this policy

	// Example: Remove from search index
	/*
	   _, err := s.searchClient.Delete().
	       Index("policies").
	       Id(policyID).
	       Do(ctx)
	   if err != nil {
	       return fmt.Errorf("failed to remove policy from search index: %w", err)
	   }
	*/

	return nil
}

// cleanupPolicyRelatedData removes any data or resources that were specifically tied to the deleted policy
func (s *PolicyService) cleanupPolicyRelatedData(ctx context.Context, policyID string) error {
	logger.Info("Cleaning up policy-related data", zap.String("policyID", policyID))

	// This is a placeholder for actual cleanup logic
	// In a real implementation, you might:
	// 1. Remove any audit logs specifically tied to this policy
	// 2. Clean up any policy-specific resources
	// 3. Update any aggregate data that included this policy

	// Example: Remove audit logs
	/*
	   if err := s.auditLogService.RemovePolicyLogs(ctx, policyID); err != nil {
	       return fmt.Errorf("failed to remove policy audit logs: %w", err)
	   }
	*/

	return nil
}
