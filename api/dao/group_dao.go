package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	echo_neo4j "github.com/dev-mohitbeniwal/echo/api/model/neo4j"
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
)

type GroupDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewGroupDAO(driver neo4j.Driver, auditService audit.Service) *GroupDAO {
	dao := &GroupDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Group", zap.Error(err))
	}
	return dao
}

func (dao *GroupDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Group ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_group_id IF NOT EXISTS
        FOR (g:` + echo_neo4j.LabelGroup + `) REQUIRE g.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Group ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Group ID")
	return nil
}

func (dao *GroupDAO) CreateGroup(ctx context.Context, group model.Group) (string, error) {
	start := time.Now()
	logger.Info("Creating new group", zap.String("groupName", group.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
			MERGE (g:` + echo_neo4j.LabelGroup + ` {id: $id})
			ON CREATE SET 
				g.name = $name,
				g.description = $description,
				g.organizationID = $organizationID,
				g.createdAt = $createdAt,
				g.updatedAt = $updatedAt
		`

		if group.DepartmentID != "" {
			query += `
				SET g.departmentID = $departmentID
				`
		}

		if len(group.Attributes) > 0 {
			query += `
				SET g.attributes = $attributes
				`
		}

		query += `
			WITH g
			MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
			MERGE (g)-[:` + echo_neo4j.RelPartOf + `]->(o)
		`

		if group.DepartmentID != "" {
			query += `
				WITH g
				MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
				MERGE (g)-[:` + echo_neo4j.RelPartOf + `]->(d)
			`
		}

		if len(group.Roles) > 0 {
			query += `
				WITH g
				UNWIND $roles AS roleID
				MATCH (r:` + echo_neo4j.LabelRole + ` {id: roleID})
				MERGE (g)-[:` + echo_neo4j.RelHasRole + `]->(r)
			`
		}

		query += `
			RETURN g.id as id
		`

		now := time.Now().Format(time.RFC3339)
		params := map[string]interface{}{
			"id":             group.ID,
			"name":           group.Name,
			"description":    group.Description,
			"organizationID": group.OrganizationID,
			"createdAt":      now,
			"updatedAt":      now,
		}

		if group.DepartmentID != "" {
			params["departmentID"] = group.DepartmentID
		}

		if len(group.Attributes) > 0 {
			// Parse attributes to JSON string
			attributesJSON, _ := json.Marshal(group.Attributes)
			params["attributes"] = string(attributesJSON)
		}

		if len(group.Roles) > 0 {
			params["roles"] = group.Roles
		}

		// Log the query and parameters
		logger.Debug("Create group query",
			zap.String("query", query),
			zap.Any("params", params))

		result, err := transaction.Run(query, params)
		if err != nil {
			logger.Error("Failed to execute create group query", zap.Error(err))
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			logger.Info("Group created successfully", zap.String("groupID", group.ID))
			logger.Info("Found group ID in result", zap.Any("result", result.Record().Values[0]))
			return result.Record().Values[0], nil
		}

		logger.Error("Failed to create group", zap.Error(err))
		logger.Error("Result is empty", zap.Any("result", result))

		return nil, echo_errors.ErrInternalServer
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create group",
			zap.Error(err),
			zap.String("groupName", group.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	groupID := fmt.Sprintf("%v", result)
	logger.Info("Group created successfully",
		zap.String("groupID", groupID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_GROUP",
		ResourceID:    groupID,
		AccessGranted: true,
		ChangeDetails: createGroupChangeDetails(nil, &group),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return groupID, nil
}

func (dao *GroupDAO) createAuditLog(ctx context.Context, action, resourceID string, oldGroup, newGroup *model.Group) error {
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        action,
		ResourceID:    resourceID,
		AccessGranted: true,
		ChangeDetails: createGroupChangeDetails(oldGroup, newGroup),
	}
	return dao.AuditService.LogAccess(ctx, auditLog)
}

func (dao *GroupDAO) UpdateGroup(ctx context.Context, group model.Group) (*model.Group, error) {
	start := time.Now()
	logger.Info("Updating group", zap.String(echo_neo4j.AttrID, group.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedGroup *model.Group
	oldGroup, err := dao.GetGroup(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	_, err = session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (g:` + echo_neo4j.LabelGroup + ` {` + echo_neo4j.AttrID + `: $id})
        SET g += $props
        WITH g
        OPTIONAL MATCH (g)-[r:` + echo_neo4j.RelPartOf + `]->(:` + echo_neo4j.LabelOrganization + `)
        DELETE r
        WITH g
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {` + echo_neo4j.AttrID + `: $organizationID})
        MERGE (g)-[:` + echo_neo4j.RelPartOf + `]->(o)
        WITH g
        OPTIONAL MATCH (g)-[r:` + echo_neo4j.RelMemberOf + `]->(:` + echo_neo4j.LabelDepartment + `)
        DELETE r
        WITH g
        `

		params := map[string]interface{}{
			echo_neo4j.AttrID: group.ID,
			"organizationID":  group.OrganizationID,
			"props": map[string]interface{}{
				echo_neo4j.AttrName:        group.Name,
				echo_neo4j.AttrDescription: group.Description,
				echo_neo4j.AttrUpdatedAt:   time.Now().Format(time.RFC3339),
			},
		}

		if group.DepartmentID != "" {
			query += `
            MATCH (d:` + echo_neo4j.LabelDepartment + ` {` + echo_neo4j.AttrID + `: $departmentID})
            MERGE (g)-[:` + echo_neo4j.RelMemberOf + `]->(d)
            `
			params["departmentID"] = group.DepartmentID
		}

		// Update Roles
		query += `
        WITH g
        OPTIONAL MATCH (g)-[r:` + echo_neo4j.RelHasRole + `]->(:` + echo_neo4j.LabelRole + `)
        DELETE r
        WITH g
        `
		if len(group.Roles) > 0 {
			query += `
            UNWIND $roles AS roleID
            MATCH (r:` + echo_neo4j.LabelRole + ` {` + echo_neo4j.AttrID + `: roleID})
            MERGE (g)-[:` + echo_neo4j.RelHasRole + `]->(r)
            WITH g
            `
			params["roles"] = group.Roles
		}

		// Update Attributes
		if len(group.Attributes) > 0 {
			query += `
            SET g.attributes = $attributes
            WITH g
            `
			params["attributes"] = group.Attributes
		} else {
			query += `
            REMOVE g.attributes
            WITH g
            `
		}

		query += `
        RETURN g
        `

		result, err := tx.Run(query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute update group query: %w", err)
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedGroup, err = mapNodeToGroup(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map group node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrGroupNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update group", zap.Error(err), zap.String(echo_neo4j.AttrID, group.ID), zap.Duration("duration", duration))
		logger.Error("Failed to update group",
			zap.Error(err),
			zap.String(echo_neo4j.AttrID, group.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Group updated successfully",
		zap.String(echo_neo4j.AttrID, group.ID),
		zap.Duration("duration", duration))

	if err := dao.createAuditLog(ctx, "UPDATE_GROUP", group.ID, oldGroup, updatedGroup); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedGroup, nil
}

func (dao *GroupDAO) DeleteGroup(ctx context.Context, groupID string) error {
	start := time.Now()
	logger.Info("Deleting group", zap.String("groupID", groupID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (g:` + echo_neo4j.LabelGroup + ` {id: $id})
        DETACH DELETE g
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": groupID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrGroupNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete group",
			zap.Error(err),
			zap.String("groupID", groupID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Group deleted successfully",
		zap.String("groupID", groupID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_GROUP",
		ResourceID:    groupID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *GroupDAO) GetGroup(ctx context.Context, groupID string) (*model.Group, error) {
	start := time.Now()
	logger.Info("Retrieving group", zap.String("groupID", groupID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (g:` + echo_neo4j.LabelGroup + ` {id: $id})
    OPTIONAL MATCH (g)-[:` + echo_neo4j.RelHasRole + `]->(r:` + echo_neo4j.LabelRole + `)
    WITH g, COLLECT(r.id) AS roleIds
    RETURN g, roleIds
    `
	result, err := session.Run(query, map[string]interface{}{"id": groupID})
	if err != nil {
		logger.Error("Failed to execute get group query",
			zap.Error(err),
			zap.String("groupID", groupID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		record := result.Record()
		node := record.Values[0].(neo4j.Node)
		roleIds := record.Values[1].([]interface{})

		group, err := mapNodeToGroup(node)
		if err != nil {
			logger.Error("Failed to map group node to struct",
				zap.Error(err),
				zap.String("groupID", groupID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}

		// Add role IDs to the group
		group.Roles = make([]string, len(roleIds))
		for i, roleID := range roleIds {
			group.Roles[i] = roleID.(string)
		}

		// Get attributes from the node
		if attrs, exists := node.Props["attributes"].(map[string]interface{}); exists {
			group.Attributes = make(map[string]string)
			for k, v := range attrs {
				group.Attributes[k] = fmt.Sprintf("%v", v)
			}
		}

		logger.Info("Group retrieved successfully",
			zap.String("groupID", groupID),
			zap.Duration("duration", time.Since(start)))
		return group, nil
	}

	logger.Warn("Group not found",
		zap.String("groupID", groupID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrGroupNotFound
}

func (dao *GroupDAO) ListGroups(ctx context.Context, limit int, offset int) ([]*model.Group, error) {
	start := time.Now()
	logger.Info("Listing groups", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (g:` + echo_neo4j.LabelGroup + `)
    RETURN g
    ORDER BY g.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `

	logger.Debug("List groups query",
		zap.String("query", query),
		zap.Int("limit", limit),
		zap.Int("offset", offset))

	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list groups query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	logger.Debug("List groups result", zap.Any("result", result))

	var groups []*model.Group
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		group, err := mapNodeToGroup(node)
		if err != nil {
			logger.Error("Failed to map group node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}

		groups = append(groups, group)
	}

	logger.Info("Groups listed successfully",
		zap.Int("count", len(groups)),
		zap.Duration("duration", time.Since(start)))

	return groups, nil
}

// Helper function to map Neo4j Node to Group struct
func mapNodeToGroup(node neo4j.Node) (*model.Group, error) {
	props := node.Props
	group := &model.Group{}

	logger := zap.L().With(zap.String("function", "mapNodeToGroup"))

	var ok bool
	var err error

	// ID
	if group.ID, ok = props[echo_neo4j.AttrID].(string); !ok {
		logger.Error("Failed to map ID", zap.Any("value", props[echo_neo4j.AttrID]))
		return nil, fmt.Errorf("invalid ID type")
	}

	// Name
	if group.Name, ok = props[echo_neo4j.AttrName].(string); !ok {
		logger.Error("Failed to map Name", zap.Any("value", props[echo_neo4j.AttrName]))
		return nil, fmt.Errorf("invalid Name type")
	}

	// Description
	if group.Description, ok = props[echo_neo4j.AttrDescription].(string); !ok {
		logger.Error("Failed to map Description", zap.Any("value", props[echo_neo4j.AttrDescription]))
		return nil, fmt.Errorf("invalid Description type")
	}

	// OrganizationID
	if group.OrganizationID, ok = props["organizationID"].(string); !ok {
		logger.Error("Failed to map OrganizationID", zap.Any("value", props["organizationID"]))
		return nil, fmt.Errorf("invalid OrganizationID type")
	}

	// DepartmentID (optional)
	if departmentID, exists := props["departmentID"]; exists {
		if group.DepartmentID, ok = departmentID.(string); !ok {
			logger.Error("Failed to map DepartmentID", zap.Any("value", departmentID))
			return nil, fmt.Errorf("invalid DepartmentID type")
		}
	}

	// Roles (optional)
	if roles, exists := props["roles"]; exists {
		rolesInterface, ok := roles.([]interface{})
		if !ok {
			logger.Error("Failed to map Roles", zap.Any("value", roles))
			return nil, fmt.Errorf("invalid Roles type")
		}
		group.Roles = make([]string, len(rolesInterface))
		for i, role := range rolesInterface {
			if group.Roles[i], ok = role.(string); !ok {
				logger.Error("Failed to map Role", zap.Any("value", role))
				return nil, fmt.Errorf("invalid Role type")
			}
		}
	}

	// Attributes (optional)
	if attributes, exists := props["attributes"]; exists {
		switch attr := attributes.(type) {
		case string:
			if err = json.Unmarshal([]byte(attr), &group.Attributes); err != nil {
				logger.Error("Failed to unmarshal Attributes", zap.Error(err))
				return nil, fmt.Errorf("failed to unmarshal Attributes: %w", err)
			}
		case map[string]interface{}:
			group.Attributes = make(map[string]string)
			for k, v := range attr {
				group.Attributes[k] = fmt.Sprintf("%v", v)
			}
		default:
			logger.Error("Invalid Attributes type", zap.Any("value", attributes))
			return nil, fmt.Errorf("invalid Attributes type")
		}
	}

	// CreatedAt
	createdAtStr, ok := props[echo_neo4j.AttrCreatedAt].(string)
	if !ok {
		logger.Error("Failed to map CreatedAt", zap.Any("value", props[echo_neo4j.AttrCreatedAt]))
		return nil, fmt.Errorf("invalid CreatedAt type")
	}
	group.CreatedAt, err = helper_util.ParseTime(createdAtStr)
	if err != nil {
		logger.Error("Failed to parse CreatedAt", zap.Error(err))
		return nil, fmt.Errorf("failed to parse CreatedAt: %w", err)
	}

	// UpdatedAt
	updatedAtStr, ok := props[echo_neo4j.AttrUpdatedAt].(string)
	if !ok {
		logger.Error("Failed to map UpdatedAt", zap.Any("value", props[echo_neo4j.AttrUpdatedAt]))
		return nil, fmt.Errorf("invalid UpdatedAt type")
	}
	group.UpdatedAt, err = helper_util.ParseTime(updatedAtStr)
	if err != nil {
		logger.Error("Failed to parse UpdatedAt", zap.Error(err))
		return nil, fmt.Errorf("failed to parse UpdatedAt: %w", err)
	}

	logger.Debug("Successfully mapped node to group", zap.Any("group", group))
	return group, nil
}

// Helper function to create change details for audit log
func createGroupChangeDetails(oldGroup, newGroup *model.Group) json.RawMessage {
	changes := make(map[string]interface{})
	if oldGroup == nil {
		changes["action"] = "created"
	} else if newGroup == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldGroup.Name != newGroup.Name {
			changes["name"] = map[string]string{"old": oldGroup.Name, "new": newGroup.Name}
		}
		if oldGroup.Description != newGroup.Description {
			changes["description"] = map[string]string{"old": oldGroup.Description, "new": newGroup.Description}
		}
		if oldGroup.OrganizationID != newGroup.OrganizationID {
			changes["organizationID"] = map[string]string{"old": oldGroup.OrganizationID, "new": newGroup.OrganizationID}
		}
		if oldGroup.DepartmentID != newGroup.DepartmentID {
			changes["departmentID"] = map[string]string{"old": oldGroup.DepartmentID, "new": newGroup.DepartmentID}
		}
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}
