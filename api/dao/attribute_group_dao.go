// api/dao/attribute_group_dao.go

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

type AttributeGroupDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewAttributeGroupDAO(driver neo4j.Driver, auditService audit.Service) *AttributeGroupDAO {
	dao := &AttributeGroupDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for AttributeGroup", zap.Error(err))
	}
	return dao
}

func (dao *AttributeGroupDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on AttributeGroup ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
		CREATE CONSTRAINT unique_attribute_group_id IF NOT EXISTS
		FOR (ag:` + echo_neo4j.LabelAttributeGroup + `) REQUIRE ag.id IS UNIQUE
		`
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on AttributeGroup ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on AttributeGroup ID")
	return nil
}

func (dao *AttributeGroupDAO) CreateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup) (string, error) {
	start := time.Now()
	logger.Info("Creating new attribute group", zap.String("name", attributeGroup.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if attributeGroup.ID == "" {
		attributeGroup.ID = uuid.New().String()
	}

	attributeGroup.CreatedAt = time.Now()
	attributeGroup.UpdatedAt = time.Now()

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		attributesJSON, err := json.Marshal(attributeGroup.Attributes)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attributes: %w", err)
		}

		query := `
        CREATE (ag:` + echo_neo4j.LabelAttributeGroup + ` {
            id: $id,
            name: $name,
            attributes: $attributes,
            createdBy: $createdBy,
            updatedBy: $updatedBy,
            createdAt: $createdAt,
            updatedAt: $updatedAt
        })
        RETURN ag.id as id
        `

		params := map[string]interface{}{
			"id":         attributeGroup.ID,
			"name":       attributeGroup.Name,
			"attributes": string(attributesJSON),
			"createdBy":  attributeGroup.CreatedBy,
			"updatedBy":  attributeGroup.UpdatedBy,
			"createdAt":  attributeGroup.CreatedAt.Format(time.RFC3339),
			"updatedAt":  attributeGroup.UpdatedAt.Format(time.RFC3339),
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, fmt.Errorf("no ID returned")
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create attribute group",
			zap.Error(err),
			zap.String("name", attributeGroup.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	attributeGroupID := fmt.Sprintf("%v", result)
	logger.Info("Attribute group created successfully",
		zap.String("attributeGroupID", attributeGroupID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_ATTRIBUTE_GROUP",
		ResourceID:    attributeGroupID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return attributeGroupID, nil
}

func (dao *AttributeGroupDAO) GetAttributeGroup(ctx context.Context, id string) (*model.AttributeGroup, error) {
	start := time.Now()
	logger.Info("Retrieving attribute group", zap.String("id", id))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (ag:` + echo_neo4j.LabelAttributeGroup + ` {id: $id})
        RETURN ag
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": id})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			return mapNodeToAttributeGroup(node)
		}

		return nil, echo_errors.ErrAttributeGroupNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to retrieve attribute group",
			zap.Error(err),
			zap.String("id", id),
			zap.Duration("duration", duration))
		return nil, err
	}

	attributeGroup := result.(*model.AttributeGroup)
	logger.Info("Attribute group retrieved successfully",
		zap.String("id", id),
		zap.Duration("duration", duration))

	return attributeGroup, nil
}

func (dao *AttributeGroupDAO) UpdateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup) (*model.AttributeGroup, error) {
	start := time.Now()
	logger.Info("Updating attribute group", zap.String("id", attributeGroup.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	attributeGroup.UpdatedAt = time.Now()

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		attributesJSON, err := json.Marshal(attributeGroup.Attributes)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attributes: %w", err)
		}

		query := `
        MATCH (ag:` + echo_neo4j.LabelAttributeGroup + ` {id: $id})
        SET ag.name = $name,
            ag.attributes = $attributes,
            ag.updatedBy = $updatedBy,
            ag.updatedAt = $updatedAt
        RETURN ag
        `

		params := map[string]interface{}{
			"id":         attributeGroup.ID,
			"name":       attributeGroup.Name,
			"attributes": string(attributesJSON),
			"updatedBy":  attributeGroup.UpdatedBy,
			"updatedAt":  attributeGroup.UpdatedAt.Format(time.RFC3339),
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, err
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			return mapNodeToAttributeGroup(node)
		}

		return nil, echo_errors.ErrAttributeGroupNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update attribute group",
			zap.Error(err),
			zap.String("id", attributeGroup.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	updatedAttributeGroup := result.(*model.AttributeGroup)
	logger.Info("Attribute group updated successfully",
		zap.String("id", updatedAttributeGroup.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_ATTRIBUTE_GROUP",
		ResourceID:    updatedAttributeGroup.ID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedAttributeGroup, nil
}

func (dao *AttributeGroupDAO) ListAttributeGroups(ctx context.Context, limit int, offset int) ([]*model.AttributeGroup, error) {
	start := time.Now()
	logger.Info("Listing attribute groups", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (ag:` + echo_neo4j.LabelAttributeGroup + `)
        RETURN ag
        ORDER BY ag.name
        SKIP $offset
        LIMIT $limit
        `
		result, err := transaction.Run(query, map[string]interface{}{
			"offset": offset,
			"limit":  limit,
		})
		if err != nil {
			return nil, err
		}

		var attributeGroups []*model.AttributeGroup
		for result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			attributeGroup, err := mapNodeToAttributeGroup(node)
			if err != nil {
				return nil, err
			}
			attributeGroups = append(attributeGroups, attributeGroup)
		}

		return attributeGroups, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to list attribute groups",
			zap.Error(err),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Duration("duration", duration))
		return nil, err
	}

	attributeGroups := result.([]*model.AttributeGroup)
	logger.Info("Attribute groups listed successfully",
		zap.Int("count", len(attributeGroups)),
		zap.Duration("duration", duration))

	return attributeGroups, nil
}

// Helper function to map Neo4j Node to AttributeGroup struct
func mapNodeToAttributeGroup(node neo4j.Node) (*model.AttributeGroup, error) {
	attributeGroup := &model.AttributeGroup{
		ID:        node.Props["id"].(string),
		Name:      node.Props["name"].(string),
		CreatedBy: node.Props["createdBy"].(string),
		UpdatedBy: node.Props["updatedBy"].(string),
	}

	attributesJSON := node.Props["attributes"].(string)
	if err := json.Unmarshal([]byte(attributesJSON), &attributeGroup.Attributes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, node.Props["createdAt"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse createdAt: %w", err)
	}
	attributeGroup.CreatedAt = createdAt

	updatedAt, err := time.Parse(time.RFC3339, node.Props["updatedAt"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse updatedAt: %w", err)
	}
	attributeGroup.UpdatedAt = updatedAt

	return attributeGroup, nil
}

func (dao *AttributeGroupDAO) DeleteAttributeGroup(ctx context.Context, id string) error {
	start := time.Now()
	logger.Info("Deleting attribute group", zap.String("id", id))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (ag:` + echo_neo4j.LabelAttributeGroup + ` {id: $id})
        DETACH DELETE ag
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": id})
		if err != nil {
			return nil, err
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, err
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrAttributeGroupNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete attribute group",
			zap.Error(err),
			zap.String("id", id),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Attribute group deleted successfully",
		zap.String("id", id),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_ATTRIBUTE_GROUP",
		ResourceID:    id,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}
