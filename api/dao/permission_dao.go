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
)

type PermissionDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewPermissionDAO(driver neo4j.Driver, auditService audit.Service) *PermissionDAO {
	dao := &PermissionDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Permission", zap.Error(err))
	}
	return dao
}

func (dao *PermissionDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Permission ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_permission_id IF NOT EXISTS
        FOR (p:` + echo_neo4j.LabelPermission + `) REQUIRE p.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Permission ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Permission ID")
	return nil
}

func (dao *PermissionDAO) CreatePermission(ctx context.Context, permission model.Permission) (string, error) {
	start := time.Now()
	logger.Info("Creating new permission", zap.String("permissionName", permission.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if permission.ID == "" {
		permission.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MERGE (p:` + echo_neo4j.LabelPermission + ` {id: $id})
        ON CREATE SET p += $props
        RETURN p.id as id
        `

		params := map[string]interface{}{
			"id": permission.ID,
			"props": map[string]interface{}{
				"name":        permission.Name,
				"description": permission.Description,
				"action":      permission.Action,
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, echo_errors.ErrInternalServer
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create permission",
			zap.Error(err),
			zap.String("permissionName", permission.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	permissionID := fmt.Sprintf("%v", result)
	logger.Info("Permission created successfully",
		zap.String("permissionID", permissionID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_" + echo_neo4j.LabelPermission,
		ResourceID:    permissionID,
		AccessGranted: true,
		ChangeDetails: createPermissionChangeDetails(nil, &permission),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return permissionID, nil
}

func (dao *PermissionDAO) UpdatePermission(ctx context.Context, permission model.Permission) (*model.Permission, error) {
	start := time.Now()
	logger.Info("Updating permission", zap.String("permissionID", permission.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedPermission *model.Permission
	oldPermission, err := dao.GetPermission(ctx, permission.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (p:` + echo_neo4j.LabelPermission + ` {id: $id})
        SET p += $props
        RETURN p
        `

		params := map[string]interface{}{
			"id": permission.ID,
			"props": map[string]interface{}{
				"name":        permission.Name,
				"description": permission.Description,
				"action":      permission.Action,
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedPermission, err = mapNodeToPermission(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map permission node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrPermissionNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update permission",
			zap.Error(err),
			zap.String("permissionID", permission.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Permission updated successfully",
		zap.String("permissionID", permission.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_" + echo_neo4j.LabelPermission,
		ResourceID:    permission.ID,
		AccessGranted: true,
		ChangeDetails: createPermissionChangeDetails(oldPermission, updatedPermission),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedPermission, nil
}

func (dao *PermissionDAO) DeletePermission(ctx context.Context, permissionID string) error {
	start := time.Now()
	logger.Info("Deleting permission", zap.String("permissionID", permissionID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (p:` + echo_neo4j.LabelPermission + ` {id: $id})
        DETACH DELETE p
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": permissionID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrPermissionNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete permission",
			zap.Error(err),
			zap.String("permissionID", permissionID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Permission deleted successfully",
		zap.String("permissionID", permissionID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_" + echo_neo4j.LabelPermission,
		ResourceID:    permissionID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *PermissionDAO) GetPermission(ctx context.Context, permissionID string) (*model.Permission, error) {
	start := time.Now()
	logger.Info("Retrieving permission", zap.String("permissionID", permissionID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:` + echo_neo4j.LabelPermission + ` {id: $id})
    RETURN p
    `
	result, err := session.Run(query, map[string]interface{}{"id": permissionID})
	if err != nil {
		logger.Error("Failed to execute get permission query",
			zap.Error(err),
			zap.String("permissionID", permissionID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		permission, err := mapNodeToPermission(node)
		if err != nil {
			logger.Error("Failed to map permission node to struct",
				zap.Error(err),
				zap.String("permissionID", permissionID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		logger.Info("Permission retrieved successfully",
			zap.String("permissionID", permissionID),
			zap.Duration("duration", time.Since(start)))
		return permission, nil
	}

	logger.Warn("Permission not found",
		zap.String("permissionID", permissionID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrPermissionNotFound
}

func (dao *PermissionDAO) ListPermissions(ctx context.Context, limit int, offset int) ([]*model.Permission, error) {
	start := time.Now()
	logger.Info("Listing permissions", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (p:` + echo_neo4j.LabelPermission + `)
    RETURN p
    ORDER BY p.name
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list permissions query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var permissions []*model.Permission
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		permission, err := mapNodeToPermission(node)
		if err != nil {
			logger.Error("Failed to map permission node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		permissions = append(permissions, permission)
	}

	logger.Info("Permissions listed successfully",
		zap.Int("count", len(permissions)),
		zap.Duration("duration", time.Since(start)))

	return permissions, nil
}

// Helper function to map Neo4j Node to Permission struct
func mapNodeToPermission(node neo4j.Node) (*model.Permission, error) {
	props := node.Props
	permission := &model.Permission{}

	var ok bool
	if permission.ID, ok = props["id"].(string); !ok {
		return nil, fmt.Errorf("invalid or missing 'id' property")
	}
	if permission.Name, ok = props[echo_neo4j.AttrName].(string); !ok {
		return nil, fmt.Errorf("invalid or missing 'name' property")
	}
	if permission.Description, ok = props[echo_neo4j.AttrDescription].(string); !ok {
		return nil, fmt.Errorf("invalid or missing 'description' property")
	}
	if permission.Action, ok = props["action"].(string); !ok {
		return nil, fmt.Errorf("invalid or missing 'action' property")
	}

	return permission, nil
}

// Helper function to create change details for audit log
func createPermissionChangeDetails(oldPermission, newPermission *model.Permission) json.RawMessage {
	changes := make(map[string]interface{})
	if oldPermission == nil {
		changes["action"] = "created"
	} else if newPermission == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldPermission.Name != newPermission.Name {
			changes["name"] = map[string]string{"old": oldPermission.Name, "new": newPermission.Name}
		}
		if oldPermission.Description != newPermission.Description {
			changes["description"] = map[string]string{"old": oldPermission.Description, "new": newPermission.Description}
		}
		if oldPermission.Action != newPermission.Action {
			changes["action"] = map[string]string{"old": oldPermission.Action, "new": newPermission.Action}
		}
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}
