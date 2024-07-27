// api/dao/policy_dao.go
package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	echo_neo4j "github.com/dev-mohitbeniwal/echo/api/model/neo4j"
)

type PolicyDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewPolicyDAO(driver neo4j.Driver, auditService audit.Service) *PolicyDAO {
	dao := &PolicyDAO{Driver: driver, AuditService: auditService}
	// Ensure unique constraint on Policy ID
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint", zap.Error(err))
	}
	return dao
}

// EnsureUniqueConstraint ensures the unique constraint on the Policy ID
func (dao *PolicyDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Policy ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() {
		if err := session.Close(); err != nil {
			logger.Error("Failed to close Neo4j session", zap.Error(err))
		}
	}()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_policy_id IF NOT EXISTS
        FOR (p:` + echo_neo4j.LabelPolicy + `) REQUIRE p.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		if err != nil {
			logger.Error("Failed to create unique constraint", zap.Error(err))
			return nil, fmt.Errorf("failed to create unique constraint: %w", err)
		}
		return nil, nil
	})
	if err != nil {
		logger.Error("Failed to ensure unique constraint on Policy ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Policy ID")
	return nil
}

// CreatePolicy creates a new policy node in Neo4j
// CreatePolicy creates a new policy node in Neo4j
func (dao *PolicyDAO) CreatePolicy(ctx context.Context, policy model.Policy, userID string) (string, error) {
	start := time.Now()
	logger.Info("Creating new policy", zap.String("policyName", policy.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if policy.ID == "" {
		policy.ID = uuid.New().String() // Generate a new UUID if ID is not provided
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		// First, check if the policy already exists
		checkQuery := `
        MATCH (p:` + echo_neo4j.LabelPolicy + ` {id: $id})
        RETURN p.id
        `
		checkResult, err := transaction.Run(checkQuery, map[string]interface{}{"id": policy.ID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}
		if checkResult.Next() {
			return nil, echo_errors.ErrPolicyConflict
		}

		// If we get here, the policy doesn't exist, so create it
		createQuery := `
            MERGE (p:` + echo_neo4j.LabelPolicy + ` {id: $id})
            ON CREATE SET p += $props
            ON MATCH SET p += $props
            RETURN p.id as id
        `

		// Convert subjects, resources, actions, and conditions to JSON strings
		subjectsJSON, _ := json.Marshal(policy.Subjects)
		resourcesJSON, _ := json.Marshal(policy.Resources)
		actionsJSON, _ := json.Marshal(policy.Actions)
		conditionsJSON, _ := json.Marshal(policy.Conditions)

		parameters := map[string]interface{}{
			"id": policy.ID,
			"props": map[string]interface{}{
				"name":             policy.Name,
				"description":      policy.Description,
				"effect":           policy.Effect,
				"priority":         policy.Priority,
				"version":          policy.Version,
				"createdAt":        time.Now().Format(time.RFC3339),
				"updatedAt":        time.Now().Format(time.RFC3339),
				"active":           policy.Active,
				"activationDate":   formatNullableTime(policy.ActivationDate),
				"deactivationDate": formatNullableTime(policy.DeactivationDate),
				"subjects":         string(subjectsJSON),
				"resources":        string(resourcesJSON),
				"actions":          string(actionsJSON),
				"conditions":       string(conditionsJSON),
			},
		}
		createResult, err := transaction.Run(createQuery, parameters)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}
		if !createResult.Next() {
			return nil, echo_errors.ErrInternalServer
		}
		id, found := createResult.Record().Get("id")
		if !found {
			return nil, echo_errors.ErrInternalServer
		}

		// Create relationships for subjects if they are users
		for _, subject := range policy.Subjects {
			if subject.Type == "user" {
				_, err = transaction.Run(`
					MATCH (u:`+echo_neo4j.LabelUser+` {id: $userID})
					MATCH (o:`+echo_neo4j.LabelOrganization+` {id: u.organizationID})
					MATCH (d:`+echo_neo4j.LabelDepartment+` {id: u.departmentID})
					MERGE (u)-[:`+echo_neo4j.RelWorksFor+`]->(o)
					MERGE (u)-[:`+echo_neo4j.RelMemberOf+`]->(d)
					WITH u
					UNWIND u.roleIds AS roleId
					MATCH (r:`+echo_neo4j.LabelRole+` {id: roleId})
					MERGE (u)-[:`+echo_neo4j.RelHasRole+`]->(r)
					WITH u
					UNWIND u.groupIds AS groupId
					MATCH (g:`+echo_neo4j.LabelGroup+` {id: groupId})
					MERGE (u)-[:`+echo_neo4j.RelBelongsToGroup+`]->(g)
				`, map[string]interface{}{
					"userID": subject.UserID,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create user relationships: %w", err)
				}
			}
		}

		return id, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create policy",
			zap.Error(err),
			zap.String("policyName", policy.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	policyID := fmt.Sprintf("%v", result)
	logger.Info("Policy created successfully",
		zap.String("policyID", policyID),
		zap.Duration("duration", duration))

	// Audit trail (unchanged)
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
				MATCH (p:` + echo_neo4j.LabelPolicy + ` {id: $id})
				SET p.name = $name, p.description = $description, p.effect = $effect,
					p.priority = $priority, p.version = $version, p.updatedAt = $updatedAt, p.createdAt = $createdAt,
					p.active = $active, p.activationDate = $activationDate, p.deactivationDate = $deactivationDate,
					p.subjects = $subjects, p.resources = $resources, p.actions = $actions, p.conditions = $conditions
				RETURN p
				`

		// Convert subjects, resources, actions, and conditions to JSON strings
		subjectsJSON, _ := json.Marshal(policy.Subjects)
		resourcesJSON, _ := json.Marshal(policy.Resources)
		actionsJSON, _ := json.Marshal(policy.Actions)
		conditionsJSON, _ := json.Marshal(policy.Conditions)

		parameters := map[string]interface{}{
			"id": policy.ID, "name": policy.Name, "description": policy.Description,
			"effect": policy.Effect, "priority": policy.Priority, "version": policy.Version,
			"updatedAt": time.Now().Format(time.RFC3339),
			"createdAt": oldPolicy.CreatedAt.Format(time.RFC3339),
			"active":    policy.Active, "activationDate": formatNullableTime(policy.ActivationDate),
			"deactivationDate": formatNullableTime(policy.DeactivationDate),
			"subjects":         string(subjectsJSON),
			"resources":        string(resourcesJSON),
			"actions":          string(actionsJSON),
			"conditions":       string(conditionsJSON),
		}
		result, err := transaction.Run(query, parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to execute update query: %w", err)
		}
		if !result.Next() {
			return nil, echo_errors.ErrPolicyNotFound
		}
		node := result.Record().Values[0].(neo4j.Node)
		updatedPolicy, err = mapNodeToPolicy(node)
		if err != nil {
			return nil, fmt.Errorf("failed to map updated policy: %w", err)
		}

		// Update relationships for subjects if they are users
		for _, subject := range policy.Subjects {
			if subject.Type == "user" {
				_, err = transaction.Run(`
					MATCH (u:`+echo_neo4j.LabelUser+` {id: $userID})
					MATCH (o:`+echo_neo4j.LabelOrganization+` {id: u.organizationID})
					MATCH (d:`+echo_neo4j.LabelDepartment+` {id: u.departmentID})
					MERGE (u)-[:`+echo_neo4j.RelWorksFor+`]->(o)
					MERGE (u)-[:`+echo_neo4j.RelMemberOf+`]->(d)
					WITH u
					UNWIND u.roleIds AS roleId
					MATCH (r:`+echo_neo4j.LabelRole+` {id: roleId})
					MERGE (u)-[:`+echo_neo4j.RelHasRole+`]->(r)
					WITH u
					UNWIND u.groupIds AS groupId
					MATCH (g:`+echo_neo4j.LabelGroup+` {id: groupId})
					MERGE (u)-[:`+echo_neo4j.RelBelongsToGroup+`]->(g)
				`, map[string]interface{}{
					"userID": subject.UserID,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to update user relationships: %w", err)
				}
			}
		}

		return nil, nil
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
        MATCH (p:` + echo_neo4j.LabelPolicy + ` {id: $id})
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
			return nil, echo_errors.ErrPolicyNotFound
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
    MATCH (p:` + echo_neo4j.LabelPolicy + ` {id: $id})
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
		policy, err := mapNodeToPolicy(node)
		if err != nil {
			logger.Error("Failed to map policy node to struct",
				zap.Error(err),
				zap.String("policyID", policyID),
				zap.Duration("duration", time.Since(start)))
			return nil, fmt.Errorf("failed to map policy node to struct: %w", err)
		}
		logger.Info("Policy retrieved successfully",
			zap.String("policyID", policyID),
			zap.Duration("duration", time.Since(start)))
		return policy, nil
	}

	logger.Warn("Policy not found",
		zap.String("policyID", policyID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrPolicyNotFound
}

// ListPolicies retrieves all policies from Neo4j with pagination
func (dao *PolicyDAO) ListPolicies(ctx context.Context, limit int, offset int) ([]*model.Policy, error) {
	start := time.Now()
	logger.Info("Listing policies", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:` + echo_neo4j.LabelPolicy + `)
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
		policy, err := mapNodeToPolicy(node)
		if err != nil {
			logger.Error("Failed to map policy node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, fmt.Errorf("failed to map policy node to struct: %w", err)
		}
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

	var queryBuilder strings.Builder
	queryBuilder.WriteString("MATCH (p:` + echo_neo4j.LabelPolicy + `) WHERE 1=1")

	params := make(map[string]interface{})

	if criteria.Name != "" {
		queryBuilder.WriteString(" AND p.name = $name")
		params["name"] = criteria.Name
	}

	if criteria.Effect != "" {
		queryBuilder.WriteString(" AND p.effect = $effect")
		params["effect"] = criteria.Effect
	}

	if criteria.MinPriority > 0 { // Assuming MinPriority should be a positive value
		queryBuilder.WriteString(" AND p.priority >= $minPriority")
		params["minPriority"] = criteria.MinPriority
	}

	if criteria.MaxPriority > 0 { // Assuming MaxPriority should be a positive value
		queryBuilder.WriteString(" AND p.priority <= $maxPriority")
		params["maxPriority"] = criteria.MaxPriority
	}

	if criteria.Active != nil {
		queryBuilder.WriteString(" AND p.active = $active")
		params["active"] = *criteria.Active
	}

	if !criteria.FromDate.IsZero() {
		queryBuilder.WriteString(" AND p.createdAt >= $fromDate")
		params["fromDate"] = criteria.FromDate.Format(time.RFC3339)
	}

	if !criteria.ToDate.IsZero() {
		queryBuilder.WriteString(" AND p.createdAt <= $toDate")
		params["toDate"] = criteria.ToDate.Format(time.RFC3339)
	}

	queryBuilder.WriteString(" RETURN p ORDER BY p.createdAt DESC")

	if criteria.Limit > 0 {
		queryBuilder.WriteString(" LIMIT $limit")
		params["limit"] = criteria.Limit
	}

	logger.Info("Executing query", zap.String("query", queryBuilder.String()), zap.Any("params", params))

	result, err := session.Run(queryBuilder.String(), params)
	if err != nil {
		logger.Error("Failed to execute search policies query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute search policies query: %w", err)
	}

	var policies []*model.Policy
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		policy, err := mapNodeToPolicy(node)
		if err != nil {
			logger.Error("Failed to map policy node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, fmt.Errorf("failed to map policy node to struct: %w", err)
		}
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
		MATCH (p:` + echo_neo4j.LabelPolicy + ` {id: $policyID})
		OPTIONAL MATCH (p)-[:` + echo_neo4j.RelAppliesTo + `]->(r:Resource)
		OPTIONAL MATCH (p)-[:` + echo_neo4j.RelAppliesTo + `]->(s:Subject)
		OPTIONAL MATCH (p)-[:` + echo_neo4j.RelHasCondition + `]->(c:Condition)
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
	return nil, echo_errors.ErrPolicyNotFound
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
func mapNodeToPolicy(node neo4j.Node) (*model.Policy, error) {
	props := node.Props
	policy := &model.Policy{}

	// ID
	if id, ok := props["id"].(string); ok {
		policy.ID = id
	} else {
		return nil, fmt.Errorf("failed to assert type for policy ID: %v", props["id"])
	}

	// Name
	if name, ok := props["name"].(string); ok {
		policy.Name = name
	} else {
		return nil, fmt.Errorf("failed to assert type for policy name: %v", props["name"])
	}

	// Description
	if description, ok := props["description"].(string); ok {
		policy.Description = description
	} else {
		return nil, fmt.Errorf("failed to assert type for policy description: %v", props["description"])
	}

	// Effect
	if effect, ok := props["effect"].(string); ok {
		if effect == echo_neo4j.PolicyEffectAllow || effect == echo_neo4j.PolicyEffectDeny {
			policy.Effect = effect
		} else {
			return nil, fmt.Errorf("invalid policy effect: %v", effect)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy effect: %v", props["effect"])
	}

	// Priority
	if priority, ok := props["priority"].(int64); ok {
		policy.Priority = int(priority)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy priority: %v", props["priority"])
	}

	// Version
	if version, ok := props["version"].(int64); ok {
		policy.Version = int(version)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy version: %v", props["version"])
	}

	// CreatedAt
	if createdAt, ok := props["createdAt"].(string); ok {
		policy.CreatedAt = parseTime(createdAt)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy createdAt: %v", props["createdAt"])
	}

	// UpdatedAt
	if updatedAt, ok := props["updatedAt"].(string); ok {
		policy.UpdatedAt = parseTime(updatedAt)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy updatedAt: %v", props["updatedAt"])
	}

	// Active
	if active, ok := props["active"].(bool); ok {
		policy.Active = active
	} else {
		return nil, fmt.Errorf("failed to assert type for policy active: %v", props["active"])
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

	// Subjects
	if subjectsJSON, ok := props["subjects"].(string); ok {
		if err := json.Unmarshal([]byte(subjectsJSON), &policy.Subjects); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy subjects: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy subjects: %v", props["subjects"])
	}

	// Resources
	if resourcesJSON, ok := props["resources"].(string); ok {
		if err := json.Unmarshal([]byte(resourcesJSON), &policy.Resources); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy resources: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy resources: %v", props["resources"])
	}

	// Actions
	if actionsJSON, ok := props["actions"].(string); ok {
		if err := json.Unmarshal([]byte(actionsJSON), &policy.Actions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy actions: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy actions: %v", props["actions"])
	}

	// Conditions
	if conditionsJSON, ok := props["conditions"].(string); ok {
		if err := json.Unmarshal([]byte(conditionsJSON), &policy.Conditions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy conditions: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy conditions: %v", props["conditions"])
	}

	return policy, nil
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
