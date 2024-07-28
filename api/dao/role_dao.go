// api/dao/role_dao.go
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

type RoleDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewRoleDAO(driver neo4j.Driver, auditService audit.Service) *RoleDAO {
	dao := &RoleDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Role", zap.Error(err))
	}
	return dao
}

func (dao *RoleDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Role ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_role_id IF NOT EXISTS
        FOR (r:` + echo_neo4j.LabelRole + `) REQUIRE r.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Role ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Role ID")
	return nil
}

func (dao *RoleDAO) CreateRole(ctx context.Context, role model.Role) (string, error) {
	start := time.Now()
	logger.Info("Creating new role", zap.String("roleName", role.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if role.ID == "" {
		role.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
			MERGE (r:` + echo_neo4j.LabelRole + ` {id: $id})
			ON CREATE SET 
				r.name = $name,
				r.description = $description,
				r.organizationID = $organizationID,
				r.createdAt = $createdAt,
				r.updatedAt = $updatedAt
		`

		if role.DepartmentID != "" {
			query += `
				SET r.departmentID = $departmentID
			`
		}

		if len(role.Attributes) > 0 {
			query += `
				SET r.attributes = $attributes
			`
		}

		query += `
			WITH r
			MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
			MERGE (r)-[:` + echo_neo4j.RelPartOf + `]->(o)
		`

		if role.DepartmentID != "" {
			query += `
				WITH r
				MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
				MERGE (r)-[:` + echo_neo4j.RelPartOf + `]->(d)
			`
		}

		if len(role.Permissions) > 0 {
			query += `
				WITH r
				UNWIND $permissions AS permissionID
				MATCH (p:` + echo_neo4j.LabelPermission + ` {id: permissionID})
				MERGE (r)-[:` + echo_neo4j.RelHasPermission + `]->(p)
			`
		}

		query += `
			RETURN r.id as id
		`

		now := time.Now().Format(time.RFC3339)
		params := map[string]interface{}{
			"id":             role.ID,
			"name":           role.Name,
			"description":    role.Description,
			"organizationID": role.OrganizationID,
			"createdAt":      now,
			"updatedAt":      now,
		}

		if role.DepartmentID != "" {
			params["departmentID"] = role.DepartmentID
		}

		if len(role.Attributes) > 0 {
			params["attributes"] = role.Attributes
		}

		if len(role.Permissions) > 0 {
			params["permissions"] = role.Permissions
		}

		// Log the query and parameters
		logger.Debug("Create role query",
			zap.String("query", query),
			zap.Any("params", params))

		result, err := transaction.Run(query, params)
		if err != nil {
			logger.Error("Failed to execute create role query", zap.Error(err))
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, echo_errors.ErrInternalServer
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create role",
			zap.Error(err),
			zap.String("roleName", role.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	roleID := fmt.Sprintf("%v", result)
	logger.Info("Role created successfully",
		zap.String("roleID", roleID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_ROLE",
		ResourceID:    roleID,
		AccessGranted: true,
		ChangeDetails: createRoleChangeDetails(nil, &role),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return roleID, nil
}

func (dao *RoleDAO) UpdateRole(ctx context.Context, role model.Role) (*model.Role, error) {
	start := time.Now()
	logger.Info("Updating role", zap.String("roleID", role.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedRole *model.Role
	oldRole, err := dao.GetRole(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (r:` + echo_neo4j.LabelRole + ` {id: $id})
        SET r += $props
        WITH r
        OPTIONAL MATCH (r)-[oldOrgRel:` + echo_neo4j.RelPartOf + `]->(:` + echo_neo4j.LabelOrganization + `)
        DELETE oldOrgRel
        WITH r
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
        MERGE (r)-[:` + echo_neo4j.RelPartOf + `]->(o)
        WITH r
        OPTIONAL MATCH (r)-[oldDeptRel:` + echo_neo4j.RelPartOf + `]->(:` + echo_neo4j.LabelDepartment + `)
        DELETE oldDeptRel
        WITH r
        OPTIONAL MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
        FOREACH (_ IN CASE WHEN d IS NOT NULL THEN [1] ELSE [] END |
            MERGE (r)-[:` + echo_neo4j.RelPartOf + `]->(d)
        )
        WITH r
        OPTIONAL MATCH (r)-[oldPermRel:` + echo_neo4j.RelHasPermission + `]->(:` + echo_neo4j.LabelPermission + `)
        DELETE oldPermRel
        WITH r
        UNWIND $permissions AS permissionID
        MATCH (p:` + echo_neo4j.LabelPermission + ` {id: permissionID})
        MERGE (r)-[:` + echo_neo4j.RelHasPermission + `]->(p)
        RETURN r
        `

		attributesJSON, _ := json.Marshal(role.Attributes)

		params := map[string]interface{}{
			"id": role.ID,
			"props": map[string]interface{}{
				echo_neo4j.AttrName:      role.Name,
				"description":            role.Description,
				"organizationID":         role.OrganizationID,
				"departmentID":           role.DepartmentID,
				"attributes":             string(attributesJSON),
				echo_neo4j.AttrUpdatedAt: time.Now().Format(time.RFC3339),
			},
			"organizationID": role.OrganizationID,
			"departmentID":   role.DepartmentID,
			"permissions":    role.Permissions,
		}

		logger.Debug("Update role query", zap.String("query", query), zap.Any("params", params))

		result, err := transaction.Run(query, params)
		if err != nil {
			// Log the error
			logger.Error("Failed to execute update role query", zap.Error(err))
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedRole, err = mapNodeToRole(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map role node to struct: %w", err)
			}

			// Fetch permissions for the updated role
			permissionsQuery := `
			MATCH (r:` + echo_neo4j.LabelRole + ` {id: $id})-[:` + echo_neo4j.RelHasPermission + `]->(p:` + echo_neo4j.LabelPermission + `)
			RETURN p.id
			`
			permissionsResult, err := transaction.Run(permissionsQuery, map[string]interface{}{"id": role.ID})
			if err != nil {
				return nil, fmt.Errorf("failed to fetch permissions: %w", err)
			}

			var permissions []string
			for permissionsResult.Next() {
				permissions = append(permissions, permissionsResult.Record().Values[0].(string))
			}
			updatedRole.Permissions = permissions

			return nil, nil
		}

		return nil, echo_errors.ErrRoleNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update role",
			zap.Error(err),
			zap.String("roleID", role.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Role updated successfully",
		zap.String("roleID", role.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_ROLE",
		ResourceID:    role.ID,
		AccessGranted: true,
		ChangeDetails: createRoleChangeDetails(oldRole, updatedRole),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedRole, nil
}

func (dao *RoleDAO) DeleteRole(ctx context.Context, roleID string) error {
	start := time.Now()
	logger.Info("Deleting role", zap.String("roleID", roleID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (r:` + echo_neo4j.LabelRole + ` {id: $id})
        DETACH DELETE r
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": roleID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrRoleNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete role",
			zap.Error(err),
			zap.String("roleID", roleID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Role deleted successfully",
		zap.String("roleID", roleID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_ROLE",
		ResourceID:    roleID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *RoleDAO) GetRole(ctx context.Context, roleID string) (*model.Role, error) {
	start := time.Now()
	logger.Info("Retrieving role", zap.String("roleID", roleID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (r:` + echo_neo4j.LabelRole + ` {id: $id})
    RETURN r
    `
	result, err := session.Run(query, map[string]interface{}{"id": roleID})
	if err != nil {
		logger.Error("Failed to execute get role query",
			zap.Error(err),
			zap.String("roleID", roleID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		role, err := mapNodeToRole(node)
		if err != nil {
			logger.Error("Failed to map role node to struct",
				zap.Error(err),
				zap.String("roleID", roleID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		logger.Info("Role retrieved successfully",
			zap.String("roleID", roleID),
			zap.Duration("duration", time.Since(start)))
		return role, nil
	}

	logger.Warn("Role not found",
		zap.String("roleID", roleID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrRoleNotFound
}

func (dao *RoleDAO) ListRoles(ctx context.Context, limit int, offset int) ([]*model.Role, error) {
	start := time.Now()
	logger.Info("Listing roles", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (r:` + echo_neo4j.LabelRole + `)
    RETURN r
    ORDER BY r.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list roles query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var roles []*model.Role
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		role, err := mapNodeToRole(node)
		if err != nil {
			logger.Error("Failed to map role node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		roles = append(roles, role)
	}

	logger.Info("Roles listed successfully",
		zap.Int("count", len(roles)),
		zap.Duration("duration", time.Since(start)))

	return roles, nil
}

func (dao *RoleDAO) AssignPermissionToRole(ctx context.Context, roleID string, permissionID string) error {
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	query := `
    MATCH (r:` + echo_neo4j.LabelRole + ` {id: $roleID})
    MATCH (p:` + echo_neo4j.LabelPermission + ` {id: $permissionID})
    MERGE (r)-[:` + echo_neo4j.RelHasPermission + `]->(p)
    `
	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		result, err := tx.Run(query, map[string]interface{}{
			"roleID":       roleID,
			"permissionID": permissionID,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume()
	})

	return err
}

func (dao *RoleDAO) GetRolePermissions(ctx context.Context, roleID string) ([]string, error) {
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (r:` + echo_neo4j.LabelRole + ` {id: $roleID})-[:` + echo_neo4j.RelHasPermission + `]->(p:` + echo_neo4j.LabelPermission + `)
    RETURN p.id
    `
	result, err := session.Run(query, map[string]interface{}{"roleID": roleID})
	if err != nil {
		return nil, err
	}

	var permissions []string
	for result.Next() {
		permissions = append(permissions, result.Record().Values[0].(string))
	}

	return permissions, nil
}

// Helper function to map Neo4j Node to Role struct
func mapNodeToRole(node neo4j.Node) (*model.Role, error) {
	props := node.Props
	role := &model.Role{}

	role.ID = props["id"].(string)
	role.Name = props["name"].(string)
	role.Description = props["description"].(string)
	role.OrganizationID = props["organizationID"].(string)
	if departmentID, ok := props["departmentID"]; ok {
		role.DepartmentID = departmentID.(string)
	}
	role.CreatedAt = helper_util.ParseTime(props["createdAt"].(string))
	role.UpdatedAt = helper_util.ParseTime(props["updatedAt"].(string))

	return role, nil
}

// Helper function to create change details for audit log
func createRoleChangeDetails(oldRole, newRole *model.Role) json.RawMessage {
	changes := make(map[string]interface{})
	if oldRole == nil {
		changes["action"] = "created"
	} else if newRole == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldRole.Name != newRole.Name {
			changes["name"] = map[string]string{"old": oldRole.Name, "new": newRole.Name}
		}
		if oldRole.Description != newRole.Description {
			changes["description"] = map[string]string{"old": oldRole.Description, "new": newRole.Description}
		}
		if oldRole.OrganizationID != newRole.OrganizationID {
			changes["organizationID"] = map[string]string{"old": oldRole.OrganizationID, "new": newRole.OrganizationID}
		}
		if oldRole.DepartmentID != newRole.DepartmentID {
			changes["departmentID"] = map[string]string{"old": oldRole.DepartmentID, "new": newRole.DepartmentID}
		}
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}
