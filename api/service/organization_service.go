// api/service/organization_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/dao"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

// IOrganizationService defines the interface for organization operations
type IOrganizationService interface {
	CreateOrganization(ctx context.Context, org model.Organization, userID string) (*model.Organization, error)
	UpdateOrganization(ctx context.Context, org model.Organization, userID string) (*model.Organization, error)
	DeleteOrganization(ctx context.Context, orgID string, userID string) error
	GetOrganization(ctx context.Context, orgID string) (*model.Organization, error)
	ListOrganizations(ctx context.Context, limit int, offset int) ([]*model.Organization, error)
	SearchOrganizations(ctx context.Context, criteria model.OrganizationSearchCriteria) ([]*model.Organization, error)
}

// OrganizationService handles business logic for organization operations
type OrganizationService struct {
	orgDAO          *dao.OrganizationDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IOrganizationService = &OrganizationService{}

// NewOrganizationService creates a new instance of OrganizationService
func NewOrganizationService(orgDAO *dao.OrganizationDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *OrganizationService {
	service := &OrganizationService{
		orgDAO:          orgDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("organization.created", service.handleOrganizationCreated)
	eventBus.Subscribe("organization.updated", service.handleOrganizationUpdated)
	eventBus.Subscribe("organization.deleted", service.handleOrganizationDeleted)

	return service
}

func (s *OrganizationService) handleOrganizationCreated(ctx context.Context, event util.Event) error {
	org := event.Payload.(model.Organization)
	logger.Info("Organization created event received", zap.String("orgID", org.ID))

	// Update any indexes or materialized views
	if err := s.updateOrganizationIndexes(ctx, org); err != nil {
		logger.Error("Failed to update organization indexes", zap.Error(err), zap.String("orgID", org.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyOrganizationChange(ctx, "created", org); err != nil {
		logger.Warn("Failed to send organization creation notification", zap.Error(err), zap.String("orgID", org.ID))
	}

	return nil
}

func (s *OrganizationService) handleOrganizationUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Organization)
	oldOrg, newOrg := payload["old"], payload["new"]

	logger.Info("Organization updated event received",
		zap.String("orgID", newOrg.ID),
		zap.Time("oldUpdatedAt", oldOrg.UpdatedAt),
		zap.Time("newUpdatedAt", newOrg.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateOrganizationIndexes(ctx, newOrg); err != nil {
		logger.Error("Failed to update organization indexes", zap.Error(err), zap.String("orgID", newOrg.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyOrganizationChange(ctx, "updated", newOrg); err != nil {
		logger.Warn("Failed to send organization update notification", zap.Error(err), zap.String("orgID", newOrg.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this organization change
	if err := s.invalidateRelatedCaches(ctx, newOrg.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("orgID", newOrg.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *OrganizationService) handleOrganizationDeleted(ctx context.Context, event util.Event) error {
	orgID := event.Payload.(string)
	logger.Info("Organization deleted event received", zap.String("orgID", orgID))

	// Remove organization from any indexes or materialized views
	if err := s.removeOrganizationFromIndexes(ctx, orgID); err != nil {
		logger.Error("Failed to remove organization from indexes", zap.Error(err), zap.String("orgID", orgID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyOrganizationChange(ctx, "deleted", model.Organization{ID: orgID}); err != nil {
		logger.Warn("Failed to send organization deletion notification", zap.Error(err), zap.String("orgID", orgID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupOrganizationRelatedData(ctx, orgID); err != nil {
		logger.Error("Failed to clean up organization-related data", zap.Error(err), zap.String("orgID", orgID))
		// Continue execution despite the error
	}

	return nil
}

// CreateOrganization handles the creation of a new organization
func (s *OrganizationService) CreateOrganization(ctx context.Context, org model.Organization, userID string) (*model.Organization, error) {
	if err := s.validationUtil.ValidateOrganization(org); err != nil {
		return nil, fmt.Errorf("invalid organization: %w", err)
	}

	// Check if organization with the same ID already exists
	if org.ID != "" {
		_, err := s.orgDAO.GetOrganization(ctx, org.ID)
		if err == nil {
			// Organization with this ID already exists
			return nil, echo_errors.ErrOrganizationConflict
		}
		if err != echo_errors.ErrOrganizationNotFound {
			// An error occurred while checking for existing organization
			return nil, echo_errors.ErrDatabaseOperation
		}
	}

	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	orgID, err := s.orgDAO.CreateOrganization(ctx, org)
	if err != nil {
		logger.Error("Error creating organization", zap.Error(err), zap.String("userID", userID))
		return nil, err
	}

	org.ID = orgID

	// Update cache
	if err := s.cacheService.SetOrganization(ctx, org); err != nil {
		logger.Warn("Failed to cache organization", zap.Error(err), zap.String("orgID", orgID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "organization.created", org)

	logger.Info("Organization created successfully", zap.String("orgID", orgID), zap.String("userID", userID))
	return &org, nil
}

// UpdateOrganization handles updates to an existing organization
func (s *OrganizationService) UpdateOrganization(ctx context.Context, org model.Organization, userID string) (*model.Organization, error) {
	if err := s.validationUtil.ValidateOrganization(org); err != nil {
		return nil, fmt.Errorf("invalid organization: %w", err)
	}

	oldOrg, err := s.orgDAO.GetOrganization(ctx, org.ID)
	if err != nil {
		logger.Error("Error retrieving existing organization", zap.Error(err), zap.String("orgID", org.ID))
		return nil, err
	}

	org.UpdatedAt = time.Now()

	updatedOrg, err := s.orgDAO.UpdateOrganization(ctx, org)
	if err != nil {
		logger.Error("Error updating organization", zap.Error(err), zap.String("orgID", org.ID), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetOrganization(ctx, *updatedOrg); err != nil {
		logger.Warn("Failed to update organization in cache", zap.Error(err), zap.String("orgID", org.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "organization.updated", map[string]model.Organization{
		"old": *oldOrg,
		"new": *updatedOrg,
	})

	logger.Info("Organization updated successfully", zap.String("orgID", org.ID), zap.String("userID", userID))
	return updatedOrg, nil
}

// DeleteOrganization handles the deletion of an organization
func (s *OrganizationService) DeleteOrganization(ctx context.Context, orgID string, userID string) error {
	err := s.orgDAO.DeleteOrganization(ctx, orgID)
	if err != nil {
		logger.Error("Error deleting organization", zap.Error(err), zap.String("orgID", orgID), zap.String("userID", userID))
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteOrganization(ctx, orgID); err != nil {
		logger.Warn("Failed to delete organization from cache", zap.Error(err), zap.String("orgID", orgID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "organization.deleted", orgID)

	logger.Info("Organization deleted successfully", zap.String("orgID", orgID), zap.String("userID", userID))
	return nil
}

// GetOrganization retrieves an organization by its ID
func (s *OrganizationService) GetOrganization(ctx context.Context, orgID string) (*model.Organization, error) {
	// Try to get from cache first
	cachedOrg, err := s.cacheService.GetOrganization(ctx, orgID)
	if err == nil && cachedOrg != nil {
		return cachedOrg, nil
	}

	org, err := s.orgDAO.GetOrganization(ctx, orgID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrOrganizationNotFound) {
			return nil, echo_errors.ErrOrganizationNotFound
		}
		logger.Error("Error retrieving organization", zap.Error(err), zap.String("orgID", orgID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetOrganization(ctx, *org); err != nil {
		logger.Warn("Failed to cache organization", zap.Error(err), zap.String("orgID", orgID))
	}

	return org, nil
}

// ListOrganizations retrieves all organizations, possibly with pagination
func (s *OrganizationService) ListOrganizations(ctx context.Context, limit int, offset int) ([]*model.Organization, error) {
	orgs, err := s.orgDAO.ListOrganizations(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing organizations", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	return orgs, nil
}

// SearchOrganizations searches for organizations based on a name pattern
func (s *OrganizationService) SearchOrganizations(ctx context.Context, criteria model.OrganizationSearchCriteria) ([]*model.Organization, error) {
	orgs, err := s.orgDAO.SearchOrganizations(ctx, criteria)
	if err != nil {
		logger.Error("Error searching organizations", zap.Error(err), zap.Any("criteria", criteria))
		return nil, fmt.Errorf("failed to search organizations: %w", err)
	}

	return orgs, nil
}

// Helper methods
func (s *OrganizationService) updateOrganizationIndexes(ctx context.Context, org model.Organization) error {
	// Implementation for updating indexes
	return nil
}

func (s *OrganizationService) invalidateRelatedCaches(ctx context.Context, orgID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *OrganizationService) removeOrganizationFromIndexes(ctx context.Context, orgID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *OrganizationService) cleanupOrganizationRelatedData(ctx context.Context, orgID string) error {
	// Implementation for cleaning up related data
	return nil
}
