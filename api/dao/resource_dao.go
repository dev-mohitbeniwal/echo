// api/dao/resource_dao.go
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
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
)

type ResourceDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewResourceDAO(driver neo4j.Driver, auditService audit.Service) *ResourceDAO {
	dao := &ResourceDAO{Driver: driver, AuditService: auditService}
	// Ensure unique constraint on Resource ID
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Resource", zap.Error(err))
	}
	return dao
}

func (dao *ResourceDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Resource ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_resource_id IF NOT EXISTS
        FOR (r:` + echo_neo4j.LabelResource + `) REQUIRE r.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Resource ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Resource ID")
	return nil
}

func (dao *ResourceDAO) CreateResource(ctx context.Context, resource model.Resource) (string, error) {
	start := time.Now()
	logger.Info("Creating new resource", zap.String("name", resource.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if resource.ID == "" {
		resource.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
            CREATE (r:` + echo_neo4j.LabelResource + ` {id: $id})
            SET r += $props
            WITH r
            MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
            CREATE (r)-[:` + echo_neo4j.RelBelongsToOrg + `]->(o)
            WITH r
            OPTIONAL MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
            FOREACH (_ IN CASE WHEN d IS NOT NULL THEN [1] ELSE [] END |
                CREATE (r)-[:` + echo_neo4j.RelBelongsToDept + `]->(d)
            )
            WITH r
            MATCH (u:` + echo_neo4j.LabelUser + ` {id: $ownerID})
            CREATE (r)-[:` + echo_neo4j.RelOwnedBy + `]->(u)
            WITH r
            MATCH (rt:` + echo_neo4j.LabelResourceType + ` {id: $typeID})
            CREATE (r)-[:` + echo_neo4j.RelHasType + `]->(rt)
            WITH r
            MATCH (ag:` + echo_neo4j.LabelAttributeGroup + ` {id: $attributeGroupID})
            CREATE (r)-[:` + echo_neo4j.RelInAttributeGroup + `]->(ag)
        `

		// Add relationships for parent and related resources
		if resource.ParentID != "" {
			query += `
                WITH r
                MATCH (p:` + echo_neo4j.LabelResource + ` {id: $parentID})
                CREATE (r)-[:` + echo_neo4j.RelChildOf + `]->(p)
            `
		}
		if len(resource.RelatedIDs) > 0 {
			query += `
                WITH r
                UNWIND $relatedIDs AS relatedID
                MATCH (related:` + echo_neo4j.LabelResource + ` {id: relatedID})
                CREATE (r)-[:` + echo_neo4j.RelRelatedTo + `]->(related)
            `
		}

		query += `
            RETURN r.id as id, r.name as name
        `

		metadataJSON, err := json.Marshal(resource.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}

		attributesJSON, err := json.Marshal(resource.Attributes)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attributes: %w", err)
		}

		now := time.Now().Format(time.RFC3339)
		params := map[string]interface{}{
			"id": resource.ID,
			"props": map[string]interface{}{
				"name":             resource.Name,
				"description":      resource.Description,
				"type":             resource.Type,
				"typeID":           resource.TypeID,
				"uri":              resource.URI,
				"organizationID":   resource.OrganizationID,
				"departmentID":     resource.DepartmentID,
				"ownerID":          resource.OwnerID,
				"status":           resource.Status,
				"version":          resource.Version,
				"tags":             resource.Tags,
				"metadata":         string(metadataJSON),
				"attributeGroupID": resource.AttributeGroupID,
				"sensitivity":      resource.Sensitivity,
				"classification":   resource.Classification,
				"location":         resource.Location,
				"format":           resource.Format,
				"size":             resource.Size,
				"createdAt":        now,
				"updatedAt":        now,
				"createdBy":        resource.CreatedBy,
				"updatedBy":        resource.UpdatedBy,
				"inheritedACL":     resource.InheritedACL,
				"attributes":       string(attributesJSON),
			},
			"organizationID":   resource.OrganizationID,
			"departmentID":     resource.DepartmentID,
			"ownerID":          resource.OwnerID,
			"typeID":           resource.TypeID,
			"attributeGroupID": resource.AttributeGroupID,
			"parentID":         resource.ParentID,
			"relatedIDs":       resource.RelatedIDs,
		}

		// Handle optional time fields
		if resource.LastAccessedAt != nil {
			params["props"].(map[string]interface{})["lastAccessedAt"] = resource.LastAccessedAt.Format(time.RFC3339)
		}
		if resource.ExpiresAt != nil {
			params["props"].(map[string]interface{})["expiresAt"] = resource.ExpiresAt.Format(time.RFC3339)
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			logger.Error("Failed to execute query", zap.Error(err))
			return nil, err
		}

		if result.Next() {
			record := result.Record()
			logger.Info("Result record", zap.Any("record", record.Values))
			id, found := record.Get("id")
			if !found {
				logger.Error("ID not found in result")
				return nil, fmt.Errorf("ID not found in result")
			}
			return id, nil
		}

		return nil, fmt.Errorf("no results returned")
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create resource",
			zap.Error(err),
			zap.String("name", resource.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	resourceID := fmt.Sprintf("%v", result)
	logger.Info("Resource created successfully",
		zap.String("resourceID", resourceID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_RESOURCE",
		ResourceID:    resourceID,
		AccessGranted: true,
		ChangeDetails: createResourceChangeDetails(nil, &resource),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return resourceID, nil
}

// Helper function to create change details for audit log
func createResourceChangeDetails(oldResource, newResource *model.Resource) json.RawMessage {
	changes := make(map[string]interface{})
	if oldResource == nil {
		changes["action"] = "created"
	} else if newResource == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldResource.Name != newResource.Name {
			changes["name"] = map[string]string{"old": oldResource.Name, "new": newResource.Name}
		}
		if oldResource.Description != newResource.Description {
			changes["description"] = map[string]string{"old": oldResource.Description, "new": newResource.Description}
		}
		// Add more fields as needed
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}

func (dao *ResourceDAO) UpdateResource(ctx context.Context, resource model.Resource) (*model.Resource, error) {
	start := time.Now()
	logger.Info("Updating resource", zap.String("resourceID", resource.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedResource *model.Resource
	oldResource, err := dao.GetResource(ctx, resource.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (r:` + echo_neo4j.LabelResource + ` {id: $id})
        SET r += $props
        WITH r
        OPTIONAL MATCH (r)-[oldOrgRel:` + echo_neo4j.RelBelongsToOrg + `]->(:` + echo_neo4j.LabelOrganization + `)
        DELETE oldOrgRel
        WITH r
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
        CREATE (r)-[:` + echo_neo4j.RelBelongsToOrg + `]->(o)
        WITH r
        OPTIONAL MATCH (r)-[oldDeptRel:` + echo_neo4j.RelBelongsToDept + `]->(:` + echo_neo4j.LabelDepartment + `)
        DELETE oldDeptRel
        WITH r
        OPTIONAL MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
        FOREACH (_ IN CASE WHEN d IS NOT NULL THEN [1] ELSE [] END |
            CREATE (r)-[:` + echo_neo4j.RelBelongsToDept + `]->(d)
        )
        WITH r
        OPTIONAL MATCH (r)-[oldOwnerRel:` + echo_neo4j.RelOwnedBy + `]->(:` + echo_neo4j.LabelUser + `)
        DELETE oldOwnerRel
        WITH r
        MATCH (u:` + echo_neo4j.LabelUser + ` {id: $ownerID})
        CREATE (r)-[:` + echo_neo4j.RelOwnedBy + `]->(u)
        WITH r
        OPTIONAL MATCH (r)-[oldTypeRel:` + echo_neo4j.RelHasType + `]->(:` + echo_neo4j.LabelResourceType + `)
        DELETE oldTypeRel
        WITH r
        MATCH (rt:` + echo_neo4j.LabelResourceType + ` {id: $typeID})
        CREATE (r)-[:` + echo_neo4j.RelHasType + `]->(rt)
        WITH r
        OPTIONAL MATCH (r)-[oldGroupRel:` + echo_neo4j.RelInAttributeGroup + `]->(:` + echo_neo4j.LabelAttributeGroup + `)
        DELETE oldGroupRel
        WITH r
        MATCH (ag:` + echo_neo4j.LabelAttributeGroup + ` {id: $attributeGroupID})
        CREATE (r)-[:` + echo_neo4j.RelInAttributeGroup + `]->(ag)
        `

		// Update parent relationship
		query += `
        WITH r
        OPTIONAL MATCH (r)-[oldParentRel:` + echo_neo4j.RelChildOf + `]->(:` + echo_neo4j.LabelResource + `)
        DELETE oldParentRel
        WITH r
        OPTIONAL MATCH (p:` + echo_neo4j.LabelResource + ` {id: $parentID})
        FOREACH (_ IN CASE WHEN p IS NOT NULL THEN [1] ELSE [] END |
            CREATE (r)-[:` + echo_neo4j.RelChildOf + `]->(p)
        )
        `

		// Update related resources
		query += `
        WITH r
        OPTIONAL MATCH (r)-[oldRelatedRel:` + echo_neo4j.RelRelatedTo + `]->(:` + echo_neo4j.LabelResource + `)
        DELETE oldRelatedRel
        WITH r
        UNWIND $relatedIDs AS relatedID
        MATCH (related:` + echo_neo4j.LabelResource + ` {id: relatedID})
        CREATE (r)-[:` + echo_neo4j.RelRelatedTo + `]->(related)
        `

		query += `
        RETURN r
        `

		metadataJSON, _ := json.Marshal(resource.Metadata)
		attributesJSON, _ := json.Marshal(resource.Attributes)

		params := map[string]interface{}{
			"id": resource.ID,
			"props": map[string]interface{}{
				"name":             resource.Name,
				"description":      resource.Description,
				"type":             resource.Type,
				"typeID":           resource.TypeID,
				"uri":              resource.URI,
				"organizationID":   resource.OrganizationID,
				"departmentID":     resource.DepartmentID,
				"ownerID":          resource.OwnerID,
				"status":           resource.Status,
				"version":          resource.Version,
				"tags":             resource.Tags,
				"metadata":         string(metadataJSON),
				"attributeGroupID": resource.AttributeGroupID,
				"sensitivity":      resource.Sensitivity,
				"classification":   resource.Classification,
				"location":         resource.Location,
				"format":           resource.Format,
				"size":             resource.Size,
				"updatedAt":        time.Now().Format(time.RFC3339),
				"updatedBy":        resource.UpdatedBy,
				"inheritedACL":     resource.InheritedACL,
				"attributes":       string(attributesJSON),
			},
			"organizationID":   resource.OrganizationID,
			"departmentID":     resource.DepartmentID,
			"ownerID":          resource.OwnerID,
			"typeID":           resource.TypeID,
			"attributeGroupID": resource.AttributeGroupID,
			"parentID":         resource.ParentID,
			"relatedIDs":       resource.RelatedIDs,
		}

		// Handle optional time fields
		if resource.LastAccessedAt != nil {
			params["props"].(map[string]interface{})["lastAccessedAt"] = resource.LastAccessedAt.Format(time.RFC3339)
		}
		if resource.ExpiresAt != nil {
			params["props"].(map[string]interface{})["expiresAt"] = resource.ExpiresAt.Format(time.RFC3339)
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			logger.Error("Failed to execute query", zap.Error(err), zap.Any("params", params))
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedResource, err = mapNodeToResource(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map resource node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrResourceNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update resource",
			zap.Error(err),
			zap.String("resourceID", resource.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Resource updated successfully",
		zap.String("resourceID", resource.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_RESOURCE",
		ResourceID:    resource.ID,
		AccessGranted: true,
		ChangeDetails: createResourceChangeDetails(oldResource, updatedResource),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedResource, nil
}

func (dao *ResourceDAO) DeleteResource(ctx context.Context, resourceID string) error {
	start := time.Now()
	logger.Info("Deleting resource", zap.String("resourceID", resourceID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (r:` + echo_neo4j.LabelResource + ` {id: $id})
        DETACH DELETE r
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": resourceID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrResourceNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete resource",
			zap.Error(err),
			zap.String("resourceID", resourceID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Resource deleted successfully",
		zap.String("resourceID", resourceID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_RESOURCE",
		ResourceID:    resourceID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *ResourceDAO) GetResource(ctx context.Context, resourceID string) (*model.Resource, error) {
	start := time.Now()
	logger.Info("Retrieving resource", zap.String("resourceID", resourceID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
		MATCH (r:` + echo_neo4j.LabelResource + ` {id: $id})
		OPTIONAL MATCH (r)-[:CHILD_OF]->(p:` + echo_neo4j.LabelResource + `)
		OPTIONAL MATCH (r)-[:RELATED_TO]->(rel:` + echo_neo4j.LabelResource + `)
		RETURN r, p.id AS parentID, COLLECT(rel.id) AS relatedIDs
	`
	result, err := session.Run(query, map[string]interface{}{"id": resourceID})
	if err != nil {
		logger.Error("Failed to execute get resource query",
			zap.Error(err),
			zap.String("resourceID", resourceID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		record := result.Record()
		node := record.Values[0].(neo4j.Node)
		parentID, _ := record.Get("parentID")
		relatedIDs, _ := record.Get("relatedIDs")

		resource, err := mapNodeToResource(node)
		if err != nil {
			logger.Error("Failed to map resource node to struct",
				zap.Error(err),
				zap.String("resourceID", resourceID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}

		if parentID != nil {
			resource.ParentID = parentID.(string)
		}

		if relatedIDs != nil {
			for _, id := range relatedIDs.([]interface{}) {
				resource.RelatedIDs = append(resource.RelatedIDs, id.(string))
			}
		}

		logger.Info("Resource retrieved successfully",
			zap.String("resourceID", resourceID),
			zap.Duration("duration", time.Since(start)))
		return resource, nil
	}

	logger.Warn("Resource not found",
		zap.String("resourceID", resourceID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrResourceNotFound
}

func (dao *ResourceDAO) ListResources(ctx context.Context, limit int, offset int) ([]*model.Resource, error) {
	start := time.Now()
	logger.Info("Listing resources", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (r:` + echo_neo4j.LabelResource + `)
    WITH r
    OPTIONAL MATCH (r)-[:BELONGS_TO]->(o:` + echo_neo4j.LabelOrganization + `)
    OPTIONAL MATCH (r)-[:ASSIGNED_TO]->(d:` + echo_neo4j.LabelDepartment + `)
    OPTIONAL MATCH (r)-[:OWNED_BY]->(u:` + echo_neo4j.LabelUser + `)
    RETURN r, o.id AS organizationID, d.id AS departmentID, u.id AS ownerID
    ORDER BY r.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `

	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list resources query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var resources []*model.Resource
	for result.Next() {
		record := result.Record()
		node := record.Values[0].(neo4j.Node)
		resource, err := mapNodeToResource(node)
		if err != nil {
			logger.Error("Failed to map resource node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}

		// Set organization, department, and owner IDs
		if organizationID, ok := record.Get("organizationID"); ok && organizationID != nil {
			resource.OrganizationID = organizationID.(string)
		}
		if departmentID, ok := record.Get("departmentID"); ok && departmentID != nil {
			resource.DepartmentID = departmentID.(string)
		}
		if ownerID, ok := record.Get("ownerID"); ok && ownerID != nil {
			resource.OwnerID = ownerID.(string)
		}

		resources = append(resources, resource)
	}

	logger.Info("Resources listed successfully",
		zap.Int("count", len(resources)),
		zap.Duration("duration", time.Since(start)))

	return resources, nil
}

// Helper function to map Neo4j Node to Resource struct
func mapNodeToResource(node neo4j.Node) (*model.Resource, error) {
	props := node.Props

	resource := &model.Resource{
		ID:               props["id"].(string),
		Name:             props["name"].(string),
		Description:      props["description"].(string),
		Type:             props["type"].(string),
		TypeID:           props["typeID"].(string),
		URI:              props["uri"].(string),
		OrganizationID:   props["organizationID"].(string),
		DepartmentID:     props["departmentID"].(string),
		OwnerID:          props["ownerID"].(string),
		Status:           props["status"].(string),
		Version:          int(props["version"].(int64)),
		AttributeGroupID: props["attributeGroupID"].(string),
		Sensitivity:      props["sensitivity"].(string),
		Classification:   props["classification"].(string),
		Location:         props["location"].(string),
		Format:           props["format"].(string),
		Size:             props["size"].(int64),
		CreatedBy:        props["createdBy"].(string),
		UpdatedBy:        props["updatedBy"].(string),
		InheritedACL:     props["inheritedACL"].(bool),
	}

	// Handle optional fields
	if tags, ok := props["tags"].([]interface{}); ok {
		for _, tag := range tags {
			resource.Tags = append(resource.Tags, tag.(string))
		}
	}

	if metadataJSON, ok := props["metadata"].(string); ok {
		if err := json.Unmarshal([]byte(metadataJSON), &resource.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource metadata: %w", err)
		}
	}

	if attributesJSON, ok := props["attributes"].(string); ok {
		if err := json.Unmarshal([]byte(attributesJSON), &resource.Attributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource attributes: %w", err)
		}
	}

	resource.CreatedAt, _ = helper_util.ParseTime(props["createdAt"].(string))
	resource.UpdatedAt, _ = helper_util.ParseTime(props["updatedAt"].(string))

	if lastAccessedAt, ok := props["lastAccessedAt"].(string); ok {
		t, _ := helper_util.ParseTime(lastAccessedAt)
		resource.LastAccessedAt = &t
	}

	if expiresAt, ok := props["expiresAt"].(string); ok {
		t, _ := helper_util.ParseTime(expiresAt)
		resource.ExpiresAt = &t
	}

	// Handle relationships
	if parentID, ok := props["parentID"].(string); ok {
		resource.ParentID = parentID
	}

	if childrenIDs, ok := props["childrenIDs"].([]interface{}); ok {
		for _, childID := range childrenIDs {
			resource.ChildrenIDs = append(resource.ChildrenIDs, childID.(string))
		}
	}

	if relatedIDs, ok := props["relatedIDs"].([]interface{}); ok {
		for _, relatedID := range relatedIDs {
			resource.RelatedIDs = append(resource.RelatedIDs, relatedID.(string))
		}
	}

	return resource, nil
}

func (dao *ResourceDAO) SearchResources(ctx context.Context, criteria model.ResourceSearchCriteria) ([]*model.Resource, error) {
	start := time.Now()
	logger.Info("Searching resources", zap.Any("criteria", criteria))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	// Build the query dynamically based on the provided criteria
	query := `MATCH (r:` + echo_neo4j.LabelResource + `)`
	whereClauses := []string{}
	params := map[string]interface{}{}

	// Add WHERE clauses for each non-empty criteria
	if criteria.ID != "" {
		whereClauses = append(whereClauses, "r.id = $id")
		params["id"] = criteria.ID
	}
	if criteria.Name != "" {
		whereClauses = append(whereClauses, "r.name CONTAINS $name")
		params["name"] = criteria.Name
	}
	if criteria.Type != "" {
		whereClauses = append(whereClauses, "r.type = $type")
		params["type"] = criteria.Type
	}
	if criteria.Status != "" {
		whereClauses = append(whereClauses, "r.status = $status")
		params["status"] = criteria.Status
	}
	if criteria.Sensitivity != "" {
		whereClauses = append(whereClauses, "r.sensitivity = $sensitivity")
		params["sensitivity"] = criteria.Sensitivity
	}
	if criteria.Classification != "" {
		whereClauses = append(whereClauses, "r.classification = $classification")
		params["classification"] = criteria.Classification
	}
	if criteria.OrganizationID != "" {
		query += ` MATCH (r)-[:BELONGS_TO]->(o:ORGANIZATION)`
		whereClauses = append(whereClauses, "o.id = $organizationId")
		params["organizationId"] = criteria.OrganizationID
	}
	if criteria.DepartmentID != "" {
		query += ` MATCH (r)-[:ASSIGNED_TO]->(d:DEPARTMENT)`
		whereClauses = append(whereClauses, "d.id = $departmentId")
		params["departmentId"] = criteria.DepartmentID
	}
	if criteria.OwnerID != "" {
		query += ` MATCH (r)-[:OWNED_BY]->(u:USER)`
		whereClauses = append(whereClauses, "u.id = $ownerId")
		params["ownerId"] = criteria.OwnerID
	}
	if len(criteria.Tags) > 0 {
		whereClauses = append(whereClauses, "ANY(tag IN r.tags WHERE tag IN $tags)")
		params["tags"] = criteria.Tags
	}
	if criteria.CreatedAfter != nil {
		whereClauses = append(whereClauses, "r.createdAt >= $createdAfter")
		params["createdAfter"] = criteria.CreatedAfter.Format(time.RFC3339)
	}
	if criteria.CreatedBefore != nil {
		whereClauses = append(whereClauses, "r.createdAt <= $createdBefore")
		params["createdBefore"] = criteria.CreatedBefore.Format(time.RFC3339)
	}
	if criteria.UpdatedAfter != nil {
		whereClauses = append(whereClauses, "r.updatedAt >= $updatedAfter")
		params["updatedAfter"] = criteria.UpdatedAfter.Format(time.RFC3339)
	}
	if criteria.UpdatedBefore != nil {
		whereClauses = append(whereClauses, "r.updatedAt <= $updatedBefore")
		params["updatedBefore"] = criteria.UpdatedBefore.Format(time.RFC3339)
	}

	// Handle custom attributes
	if len(criteria.Attributes) > 0 {
		for key, value := range criteria.Attributes {
			attrKey := "attr_" + key
			whereClauses = append(whereClauses, "r.attributes CONTAINS $"+attrKey)
			params[attrKey] = fmt.Sprintf(`"%s":"%v"`, key, value)
		}
	}

	// Add WHERE clause if any conditions exist
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Add WITH clause
	query += " WITH r"

	// Add ORDER BY clause
	if criteria.SortBy != "" {
		query += ` ORDER BY r.` + criteria.SortBy
		if strings.ToLower(criteria.SortOrder) == "desc" {
			query += " DESC"
		} else {
			query += " ASC"
		}
	} else {
		query += " ORDER BY r.createdAt DESC"
	}

	// Add SKIP and LIMIT clauses
	query += ` SKIP $offset LIMIT $limit`

	// Add RETURN clause
	query += " RETURN r"
	params["offset"] = criteria.Offset
	params["limit"] = criteria.Limit

	// Log the query
	logger.Debug("Search resources query", zap.String("query", query), zap.Any("params", params))

	// Execute the query
	result, err := session.Run(query, params)
	if err != nil {
		logger.Error("Failed to execute search resources query",
			zap.Error(err),
			zap.String("query", query),
			zap.Any("params", params),
			zap.Duration("duration", time.Since(start)))
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	var resources []*model.Resource
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		resource, err := mapNodeToResource(node)
		if err != nil {
			logger.Error("Failed to map resource node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		resources = append(resources, resource)
	}

	logger.Info("Resources searched successfully",
		zap.Int("count", len(resources)),
		zap.Duration("duration", time.Since(start)))

	return resources, nil
}
