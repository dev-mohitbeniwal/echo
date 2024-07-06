// api/dao/policy_dao.go
package dao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
)

// Custom error types
var (
	ErrPolicyNotFound    = errors.New("policy not found")
	ErrDatabaseOperation = errors.New("database operation failed")
)

type PolicyDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewPolicyDAO(driver neo4j.Driver, auditService audit.Service) *PolicyDAO {
	return &PolicyDAO{Driver: driver, AuditService: auditService}
}

// CreatePolicy creates a new policy node in Neo4j
func (dao *PolicyDAO) CreatePolicy(ctx context.Context, policy model.Policy, userID string) (string, error) {
	start := time.Now()
	logger.Info("Creating new policy", zap.String("policyName", policy.Name))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE (p:Policy {
            id: $id, name: $name, description: $description, effect: $effect,
            priority: $priority, version: $version, createdAt: $createdAt, updatedAt: $updatedAt,
            active: $active, activationDate: $activationDate, deactivationDate: $deactivationDate
        })
        RETURN p.id
        `
		parameters := map[string]interface{}{
			"id": policy.ID, "name": policy.Name, "description": policy.Description,
			"effect": policy.Effect, "priority": policy.Priority, "version": policy.Version,
			"createdAt": time.Now().Format(time.RFC3339), "updatedAt": time.Now().Format(time.RFC3339),
			"active": policy.Active, "activationDate": formatNullableTime(policy.ActivationDate),
			"deactivationDate": formatNullableTime(policy.DeactivationDate),
		}
		result, err := transaction.Run(query, parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to execute create query: %w", err)
		}
		if result.Next() {
			return result.Record().Values[0], nil
		}
		return nil, errors.New("no id returned after policy creation")
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create policy",
			zap.Error(err),
			zap.String("policyName", policy.Name),
			zap.Duration("duration", duration))
		return "", fmt.Errorf("failed to create policy: %w", err)
	}

	policyID := result.(string)
	logger.Info("Policy created successfully",
		zap.String("policyID", policyID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        userID,
		Action:        "CREATE_POLICY",
		ResourceID:    policyID,
		AccessGranted: true,
		PolicyID:      policyID,
		ChangeDetails: createChangeDetails(nil, &policy),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return policyID, nil
}

// UpdatePolicy updates an existing policy in Neo4j
func (dao *PolicyDAO) UpdatePolicy(ctx context.Context, policy model.Policy, userID string) (*model.Policy, error) {
	start := time.Now()
	logger.Info("Updating policy", zap.String("policyID", policy.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedPolicy *model.Policy
	oldPolicy, err := dao.GetPolicy(ctx, policy.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (p:Policy {id: $id})
        SET p.name = $name, p.description = $description, p.effect = $effect,
            p.priority = $priority, p.version = $version, p.updatedAt = $updatedAt,
            p.active = $active, p.activationDate = $activationDate, p.deactivationDate = $deactivationDate
        RETURN p
        `
		parameters := map[string]interface{}{
			"id": policy.ID, "name": policy.Name, "description": policy.Description,
			"effect": policy.Effect, "priority": policy.Priority, "version": policy.Version,
			"updatedAt": time.Now().Format(time.RFC3339),
			"active":    policy.Active, "activationDate": formatNullableTime(policy.ActivationDate),
			"deactivationDate": formatNullableTime(policy.DeactivationDate),
		}
		result, err := transaction.Run(query, parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to execute update query: %w", err)
		}
		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedPolicy = mapNodeToPolicy(node)
			return nil, nil
		}
		return nil, ErrPolicyNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update policy",
			zap.Error(err),
			zap.String("policyID", policy.ID),
			zap.Duration("duration", duration))
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	logger.Info("Policy updated successfully",
		zap.String("policyID", policy.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        userID,
		Action:        "UPDATE_POLICY",
		ResourceID:    policy.ID,
		AccessGranted: true,
		PolicyID:      policy.ID,
		ChangeDetails: createChangeDetails(oldPolicy, updatedPolicy),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedPolicy, nil
}

// DeletePolicy deletes a policy from Neo4j
func (dao *PolicyDAO) DeletePolicy(ctx context.Context, policyID string, userID string) error {
	start := time.Now()
	logger.Info("Deleting policy", zap.String("policyID", policyID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (p:Policy {id: $id})
        DETACH DELETE p
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": policyID})
		if err != nil {
			return nil, fmt.Errorf("failed to execute delete query: %w", err)
		}
		summary, err := result.Consume()
		if err != nil {
			return nil, fmt.Errorf("failed to consume delete result: %w", err)
		}
		if summary.Counters().NodesDeleted() == 0 {
			return nil, ErrPolicyNotFound
		}
		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete policy",
			zap.Error(err),
			zap.String("policyID", policyID),
			zap.Duration("duration", duration))
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	logger.Info("Policy deleted successfully",
		zap.String("policyID", policyID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        userID,
		Action:        "DELETE_POLICY",
		ResourceID:    policyID,
		AccessGranted: true,
		PolicyID:      policyID,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

// GetPolicy retrieves a policy from Neo4j by its ID
func (dao *PolicyDAO) GetPolicy(ctx context.Context, policyID string) (*model.Policy, error) {
	start := time.Now()
	logger.Info("Retrieving policy", zap.String("policyID", policyID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:Policy {id: $id})
    RETURN p
    `
	result, err := session.Run(query, map[string]interface{}{"id": policyID})
	if err != nil {
		logger.Error("Failed to execute get policy query",
			zap.Error(err),
			zap.String("policyID", policyID),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute get policy query: %w", err)
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		policy := mapNodeToPolicy(node)
		logger.Info("Policy retrieved successfully",
			zap.String("policyID", policyID),
			zap.Duration("duration", time.Since(start)))
		return policy, nil
	}

	logger.Warn("Policy not found",
		zap.String("policyID", policyID),
		zap.Duration("duration", time.Since(start)))
	return nil, ErrPolicyNotFound
}

// ListPolicies retrieves all policies from Neo4j with pagination
func (dao *PolicyDAO) ListPolicies(ctx context.Context, limit int, offset int) ([]*model.Policy, error) {
	start := time.Now()
	logger.Info("Listing policies", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:Policy)
    RETURN p
    ORDER BY p.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list policies query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute list policies query: %w", err)
	}

	var policies []*model.Policy
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		policy := mapNodeToPolicy(node)
		policies = append(policies, policy)
	}

	logger.Info("Policies listed successfully",
		zap.Int("count", len(policies)),
		zap.Duration("duration", time.Since(start)))

	return policies, nil
}

// SearchPolicies searches for policies based on given criteria
func (dao *PolicyDAO) SearchPolicies(ctx context.Context, criteria model.PolicySearchCriteria) ([]*model.Policy, error) {
	start := time.Now()
	logger.Info("Searching policies", zap.Any("criteria", criteria))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:Policy)
    WHERE 
        ($name = '' OR p.name CONTAINS $name)
        AND ($effect = '' OR p.effect = $effect)
        AND ($minPriority = -1 OR p.priority >= $minPriority)
        AND ($maxPriority = -1 OR p.priority <= $maxPriority)
        AND ($active IS NULL OR p.active = $active)
        AND ($fromDate = '' OR p.createdAt >= $fromDate)
        AND ($toDate = '' OR p.createdAt <= $toDate)
    RETURN p
    ORDER BY p.createdAt DESC
    LIMIT $limit
    `

	params := map[string]interface{}{
		"name":        criteria.Name,
		"effect":      criteria.Effect,
		"minPriority": criteria.MinPriority,
		"maxPriority": criteria.MaxPriority,
		"active":      criteria.Active,
		"fromDate":    criteria.FromDate.Format(time.RFC3339),
		"toDate":      criteria.ToDate.Format(time.RFC3339),
		"limit":       criteria.Limit,
	}

	result, err := session.Run(query, params)
	if err != nil {
		logger.Error("Failed to execute search policies query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute search policies query: %w", err)
	}

	var policies []*model.Policy
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		policy := mapNodeToPolicy(node)
		policies = append(policies, policy)
	}

	logger.Info("Policies searched successfully",
		zap.Int("count", len(policies)),
		zap.Duration("duration", time.Since(start)))

	return policies, nil
}

// AnalyzePolicyUsage analyzes the usage of a policy
func (dao *PolicyDAO) AnalyzePolicyUsage(ctx context.Context, policyID string) (*model.PolicyUsageAnalysis, error) {
	start := time.Now()
	logger.Info("Analyzing policy usage", zap.String("policyID", policyID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:Policy {id: $policyID})
    OPTIONAL MATCH (p)-[:APPLIES_TO]->(r:Resource)
    OPTIONAL MATCH (p)-[:APPLIES_TO]->(s:Subject)
    OPTIONAL MATCH (p)-[:HAS_CONDITION]->(c:Condition)
    RETURN 
        p.id AS policyID,
        p.name AS policyName,
        COUNT(DISTINCT r) AS resourceCount,
        COUNT(DISTINCT s) AS subjectCount,
        COUNT(DISTINCT c) AS conditionCount,
        p.createdAt AS createdAt,
        p.updatedAt AS updatedAt
    `

	result, err := session.Run(query, map[string]interface{}{"policyID": policyID})
	if err != nil {
		logger.Error("Failed to execute analyze policy usage query",
			zap.Error(err),
			zap.String("policyID", policyID),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute analyze policy usage query: %w", err)
	}

	if result.Next() {
		record := result.Record()
		analysis := &model.PolicyUsageAnalysis{
			PolicyID:       record.Values[0].(string),
			PolicyName:     record.Values[1].(string),
			ResourceCount:  int(record.Values[2].(int64)),
			SubjectCount:   int(record.Values[3].(int64)),
			ConditionCount: int(record.Values[4].(int64)),
			CreatedAt:      parseTime(record.Values[5].(string)),
			LastUpdatedAt:  parseTime(record.Values[6].(string)),
		}

		logger.Info("Policy usage analyzed successfully",
			zap.String("policyID", policyID),
			zap.Duration("duration", time.Since(start)))

		return analysis, nil
	}

	logger.Warn("Policy not found for usage analysis",
		zap.String("policyID", policyID),
		zap.Duration("duration", time.Since(start)))
	return nil, fmt.Errorf("policy not found: %s", policyID)
}

// Helper function to create change details for audit log
func createChangeDetails(oldPolicy, newPolicy *model.Policy) json.RawMessage {
	changes := make(map[string]interface{})
	if oldPolicy == nil {
		changes["action"] = "created"
	} else if newPolicy == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldPolicy.Name != newPolicy.Name {
			changes["name"] = map[string]string{"old": oldPolicy.Name, "new": newPolicy.Name}
		}
		// Add more fields as needed
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}

// Helper function to map Neo4j Node to Policy struct
func mapNodeToPolicy(node neo4j.Node) *model.Policy {
	props := node.Props
	policy := &model.Policy{}

	// ID
	if id, ok := props["id"].(string); ok {
		policy.ID = id
	} else {
		logger.Error("Failed to assert type for policy ID", zap.Any("ID", props["id"]))
	}

	// Name
	if name, ok := props["name"].(string); ok {
		policy.Name = name
	} else {
		logger.Error("Failed to assert type for policy name", zap.Any("Name", props["name"]))
	}

	// Description
	if description, ok := props["description"].(string); ok {
		policy.Description = description
	} else {
		logger.Error("Failed to assert type for policy description", zap.Any("Description", props["description"]))
	}

	// Effect
	if effect, ok := props["effect"].(string); ok {
		policy.Effect = effect
	} else {
		logger.Error("Failed to assert type for policy effect", zap.Any("Effect", props["effect"]))
	}

	// Priority
	if priority, ok := props["priority"].(int64); ok {
		policy.Priority = int(priority)
	} else {
		logger.Error("Failed to assert type for policy priority", zap.Any("Priority", props["priority"]))
	}

	// Version
	if version, ok := props["version"].(int64); ok {
		policy.Version = int(version)
	} else {
		logger.Error("Failed to assert type for policy version", zap.Any("Version", props["version"]))
	}

	// CreatedAt
	if createdAt, ok := props["createdAt"].(string); ok {
		policy.CreatedAt = parseTime(createdAt)
	} else {
		logger.Error("Failed to assert type for policy createdAt", zap.Any("CreatedAt", props["createdAt"]))
	}

	// UpdatedAt
	if updatedAt, ok := props["updatedAt"].(string); ok {
		policy.UpdatedAt = parseTime(updatedAt)
	} else {
		logger.Error("Failed to assert type for policy updatedAt", zap.Any("UpdatedAt", props["updatedAt"]))
	}

	// Active
	if active, ok := props["active"].(bool); ok {
		policy.Active = active
	} else {
		logger.Error("Failed to assert type for policy active", zap.Any("Active", props["active"]))
	}

	// ActivationDate
	if activationDate, ok := props["activationDate"]; ok {
		policy.ActivationDate = parseNullableTime(activationDate)
	} else {
		logger.Warn("Activation date not found or null", zap.Any("ActivationDate", props["activationDate"]))
	}

	// DeactivationDate
	if deactivationDate, ok := props["deactivationDate"]; ok {
		policy.DeactivationDate = parseNullableTime(deactivationDate)
	} else {
		logger.Warn("Deactivation date not found or null", zap.Any("DeactivationDate", props["deactivationDate"]))
	}

	return policy
}

// Helper function to parse time
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// Helper function to parse nullable time
func parseNullableTime(v interface{}) *time.Time {
	if s, ok := v.(string); ok {
		t, _ := time.Parse(time.RFC3339, s)
		return &t
	}
	return nil
}

// Helper function to format nullable time
func formatNullableTime(t *time.Time) interface{} {
	if t != nil {
		return t.Format(time.RFC3339)
	}
	return nil
}
