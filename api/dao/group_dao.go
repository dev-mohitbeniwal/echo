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
	logger.Info("Creating new group", zap.String(echo_neo4j.AttrName, group.Name))

	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	result, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		query := `
        MERGE (g:` + echo_neo4j.LabelGroup + ` {` + echo_neo4j.AttrID + `: $id})
        ON CREATE SET g += $props
        WITH g
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {` + echo_neo4j.AttrID + `: $organizationID})
        MERGE (g)-[:` + echo_neo4j.RelPartOf + `]->(o)
        RETURN g.` + echo_neo4j.AttrID + ` as id
        `

		params := map[string]interface{}{
			echo_neo4j.AttrID: group.ID,
			"organizationID":  group.OrganizationID,
			"props": map[string]interface{}{
				echo_neo4j.AttrName:        group.Name,
				echo_neo4j.AttrDescription: group.Description,
				echo_neo4j.AttrCreatedAt:   time.Now().Format(time.RFC3339),
				echo_neo4j.AttrUpdatedAt:   time.Now().Format(time.RFC3339),
			},
		}

		if group.DepartmentID != "" {
			query += `
            WITH g
            MATCH (d:` + echo_neo4j.LabelDepartment + ` {` + echo_neo4j.AttrID + `: $departmentID})
            MERGE (g)-[:` + echo_neo4j.RelMemberOf + `]->(d)
            `
			params["departmentID"] = group.DepartmentID
		}

		result, err := tx.Run(query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute create group query: %w", err)
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, echo_errors.ErrInternalServer
	})

	if err != nil {
		logger.Error("Failed to create group",
			zap.Error(err),
			zap.String(echo_neo4j.AttrName, group.Name),
			zap.Duration("duration", time.Since(start)))
		return "", err
	}

	groupID := fmt.Sprintf("%v", result)
	logger.Info("Group created successfully",
		zap.String(echo_neo4j.AttrID, groupID),
		zap.Duration("duration", time.Since(start)))

	if err := dao.createAuditLog(ctx, "CREATE_GROUP", groupID, nil, &group); err != nil {
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
    RETURN g
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
		node := result.Record().Values[0].(neo4j.Node)
		group, err := mapNodeToGroup(node)
		if err != nil {
			logger.Error("Failed to map group node to struct",
				zap.Error(err),
				zap.String("groupID", groupID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
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

	group.ID = props[echo_neo4j.AttrID].(string)
	group.Name = props[echo_neo4j.AttrName].(string)
	group.Description = props[echo_neo4j.AttrDescription].(string)
	group.OrganizationID = props["organizationID"].(string)
	if departmentID, ok := props["departmentID"]; ok {
		group.DepartmentID = departmentID.(string)
	}
	group.CreatedAt = helper_util.ParseTime(props[echo_neo4j.AttrCreatedAt].(string))
	group.UpdatedAt = helper_util.ParseTime(props[echo_neo4j.AttrUpdatedAt].(string))

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
