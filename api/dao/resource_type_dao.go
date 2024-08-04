// api/dao/resource_type_dao.go

package dao

import (
	"context"
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

type ResourceTypeDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewResourceTypeDAO(driver neo4j.Driver, auditService audit.Service) *ResourceTypeDAO {
	dao := &ResourceTypeDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for ResourceType", zap.Error(err))
	}
	return dao
}

func (dao *ResourceTypeDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on ResourceType ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
		CREATE CONSTRAINT unique_resource_type_id IF NOT EXISTS
		FOR (rt:` + echo_neo4j.LabelResourceType + `) REQUIRE rt.id IS UNIQUE
		`
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on ResourceType ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on ResourceType ID")
	return nil
}

func (dao *ResourceTypeDAO) CreateResourceType(ctx context.Context, resourceType model.ResourceType) (string, error) {
	start := time.Now()
	logger.Info("Creating new resource type", zap.String("name", resourceType.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if resourceType.ID == "" {
		resourceType.ID = uuid.New().String()
	}

	resourceType.CreatedAt = time.Now()
	resourceType.UpdatedAt = time.Now()

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE (rt:` + echo_neo4j.LabelResourceType + ` {
            id: $id,
            name: $name,
            description: $description,
            createdBy: $createdBy,
            updatedBy: $updatedBy,
            createdAt: $createdAt,
            updatedAt: $updatedAt
        })
        RETURN rt.id as id
        `

		params := map[string]interface{}{
			"id":          resourceType.ID,
			"name":        resourceType.Name,
			"description": resourceType.Description,
			"createdBy":   resourceType.CreatedBy,
			"updatedBy":   resourceType.UpdatedBy,
			"createdAt":   resourceType.CreatedAt.Format(time.RFC3339),
			"updatedAt":   resourceType.UpdatedAt.Format(time.RFC3339),
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
		logger.Error("Failed to create resource type",
			zap.Error(err),
			zap.String("name", resourceType.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	resourceTypeID := fmt.Sprintf("%v", result)
	logger.Info("Resource type created successfully",
		zap.String("resourceTypeID", resourceTypeID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_RESOURCE_TYPE",
		ResourceID:    resourceTypeID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return resourceTypeID, nil
}

func (dao *ResourceTypeDAO) UpdateResourceType(ctx context.Context, resourceType model.ResourceType) (*model.ResourceType, error) {
	start := time.Now()
	logger.Info("Updating resource type", zap.String("id", resourceType.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	resourceType.UpdatedAt = time.Now()

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (rt:` + echo_neo4j.LabelResourceType + ` {id: $id})
        SET rt.name = $name,
            rt.description = $description,
            rt.updatedBy = $updatedBy,
            rt.updatedAt = $updatedAt
        RETURN rt
        `

		params := map[string]interface{}{
			"id":          resourceType.ID,
			"name":        resourceType.Name,
			"description": resourceType.Description,
			"updatedBy":   resourceType.UpdatedBy,
			"updatedAt":   resourceType.UpdatedAt.Format(time.RFC3339),
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, err
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			return mapNodeToResourceType(node)
		}

		return nil, echo_errors.ErrResourceTypeNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update resource type",
			zap.Error(err),
			zap.String("id", resourceType.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	updatedResourceType := result.(*model.ResourceType)
	logger.Info("Resource type updated successfully",
		zap.String("id", updatedResourceType.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_RESOURCE_TYPE",
		ResourceID:    updatedResourceType.ID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedResourceType, nil
}

func (dao *ResourceTypeDAO) GetResourceType(ctx context.Context, id string) (*model.ResourceType, error) {
	start := time.Now()
	logger.Info("Retrieving resource type", zap.String("id", id))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (rt:` + echo_neo4j.LabelResourceType + ` {id: $id})
        RETURN rt
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": id})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			return mapNodeToResourceType(node)
		}

		return nil, echo_errors.ErrResourceTypeNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to retrieve resource type",
			zap.Error(err),
			zap.String("id", id),
			zap.Duration("duration", duration))
		return nil, err
	}

	resourceType := result.(*model.ResourceType)
	logger.Info("Resource type retrieved successfully",
		zap.String("id", id),
		zap.Duration("duration", duration))

	return resourceType, nil
}

func (dao *ResourceTypeDAO) ListResourceTypes(ctx context.Context, limit int, offset int) ([]*model.ResourceType, error) {
	start := time.Now()
	logger.Info("Listing resource types", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (rt:` + echo_neo4j.LabelResourceType + `)
        RETURN rt
        ORDER BY rt.name
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

		var resourceTypes []*model.ResourceType
		for result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			resourceType, err := mapNodeToResourceType(node)
			if err != nil {
				return nil, err
			}
			resourceTypes = append(resourceTypes, resourceType)
		}

		return resourceTypes, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to list resource types",
			zap.Error(err),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Duration("duration", duration))
		return nil, err
	}

	resourceTypes := result.([]*model.ResourceType)
	logger.Info("Resource types listed successfully",
		zap.Int("count", len(resourceTypes)),
		zap.Duration("duration", duration))

	return resourceTypes, nil
}

// Helper function to map Neo4j Node to ResourceType struct
func mapNodeToResourceType(node neo4j.Node) (*model.ResourceType, error) {
	createdAt, err := time.Parse(time.RFC3339, node.Props["createdAt"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse createdAt: %w", err)
	}

	updatedAt, err := time.Parse(time.RFC3339, node.Props["updatedAt"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse updatedAt: %w", err)
	}

	return &model.ResourceType{
		ID:          node.Props["id"].(string),
		Name:        node.Props["name"].(string),
		Description: node.Props["description"].(string),
		CreatedBy:   node.Props["createdBy"].(string),
		UpdatedBy:   node.Props["updatedBy"].(string),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (dao *ResourceTypeDAO) DeleteResourceType(ctx context.Context, id string) error {
	start := time.Now()
	logger.Info("Deleting resource type", zap.String("id", id))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (rt:` + echo_neo4j.LabelResourceType + ` {id: $id})
        DETACH DELETE rt
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
			return nil, echo_errors.ErrResourceTypeNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete resource type",
			zap.Error(err),
			zap.String("id", id),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Resource type deleted successfully",
		zap.String("id", id),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_RESOURCE_TYPE",
		ResourceID:    id,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}
