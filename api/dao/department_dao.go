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
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
)

type DepartmentDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewDepartmentDAO(driver neo4j.Driver, auditService audit.Service) *DepartmentDAO {
	dao := &DepartmentDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Department", zap.Error(err))
	}
	return dao
}

func (dao *DepartmentDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Department ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_dept_id IF NOT EXISTS
        FOR (d:DEPARTMENT) REQUIRE d.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Department ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Department ID")
	return nil
}

func (dao *DepartmentDAO) CreateDepartment(ctx context.Context, department model.Department) (string, error) {
	start := time.Now()
	logger.Info("Creating new department", zap.String("deptName", department.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if department.ID == "" {
		department.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE (d:DEPARTMENT {id: $id, name: $name, organizationID: $orgId, parentID: $parentId, createdAt: $createdAt, updatedAt: $updatedAt})
        WITH d
        MATCH (o:ORGANIZATION {id: $orgId})
        CREATE (d)-[:BELONGS_TO]->(o)
        WITH d, o
        OPTIONAL MATCH (parent:DEPARTMENT {id: $parentId})
        FOREACH (_ IN CASE WHEN parent IS NOT NULL THEN [1] ELSE [] END |
            CREATE (d)-[:CHILD_OF]->(parent)
        )
        RETURN d.id as id
        `

		params := map[string]interface{}{
			"id":        department.ID,
			"name":      department.Name,
			"orgId":     department.OrganizationID,
			"parentId":  department.ParentID,
			"createdAt": time.Now().Format(time.RFC3339),
			"updatedAt": time.Now().Format(time.RFC3339),
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, fmt.Errorf("no id returned")
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create department",
			zap.Error(err),
			zap.String("deptName", department.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	deptID := fmt.Sprintf("%v", result)
	logger.Info("Department created successfully",
		zap.String("deptID", deptID),
		zap.Duration("duration", duration))

	// Verify relationship creation
	verifyErr := dao.verifyRelationships(ctx, deptID, department.OrganizationID, department.ParentID)
	if verifyErr != nil {
		logger.Error("Failed to verify relationships",
			zap.Error(verifyErr),
			zap.String("deptID", deptID),
			zap.String("orgID", department.OrganizationID))
	}

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_DEPARTMENT",
		ResourceID:    deptID,
		AccessGranted: true,
		ChangeDetails: createDeptChangeDetails(nil, &department),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return deptID, nil
}

func (dao *DepartmentDAO) verifyRelationships(ctx context.Context, deptID, orgID, parentID string) error {
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (d:DEPARTMENT {id: $deptID})
    OPTIONAL MATCH (d)-[r:BELONGS_TO]->(o:ORGANIZATION)
    OPTIONAL MATCH (d)-[p:CHILD_OF]->(parent:DEPARTMENT)
    RETURN r, p, o.id as orgId, parent.id as parentId
    `

	result, err := session.Run(query, map[string]interface{}{
		"deptID": deptID,
	})
	if err != nil {
		return fmt.Errorf("failed to verify relationships: %w", err)
	}

	if result.Next() {
		record := result.Record()
		orgRel, _ := record.Get("r")
		parentRel, _ := record.Get("p")
		returnedOrgID, _ := record.Get("orgId")
		returnedParentID, _ := record.Get("parentId")

		if orgRel == nil {
			logger.Error("BELONGS_TO relationship not created", zap.String("deptID", deptID), zap.String("orgID", orgID))
		} else {
			logger.Info("BELONGS_TO relationship verified", zap.String("deptID", deptID), zap.String("orgID", returnedOrgID.(string)))
		}

		if parentID != "" && parentRel == nil {
			logger.Error("CHILD_OF relationship not created", zap.String("deptID", deptID), zap.String("parentID", parentID))
		} else if parentID != "" {
			logger.Info("CHILD_OF relationship verified", zap.String("deptID", deptID), zap.String("parentID", returnedParentID.(string)))
		}
	} else {
		return fmt.Errorf("no results returned when verifying relationships")
	}

	return nil
}

func (dao *DepartmentDAO) UpdateDepartment(ctx context.Context, department model.Department) (*model.Department, error) {
	start := time.Now()
	logger.Info("Updating department", zap.String("deptID", department.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedDept *model.Department
	oldDept, err := dao.GetDepartment(ctx, department.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (d:DEPARTMENT {id: $id})
        SET d += $props
        WITH d
        MATCH (o:ORGANIZATION {id: $orgId})
        MERGE (d)-[:BELONGS_TO]->(o)
        RETURN d
        `

		params := map[string]interface{}{
			"id":    department.ID,
			"orgId": department.OrganizationID,
			"props": map[string]interface{}{
				"name":           department.Name,
				"organizationID": department.OrganizationID,
				"parentID":       department.ParentID,
				"updatedAt":      time.Now().Format(time.RFC3339),
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedDept, err = mapNodeToDepartment(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map department node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrDepartmentNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update department",
			zap.Error(err),
			zap.String("deptID", department.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Department updated successfully",
		zap.String("deptID", department.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_DEPARTMENT",
		ResourceID:    department.ID,
		AccessGranted: true,
		ChangeDetails: createDeptChangeDetails(oldDept, updatedDept),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedDept, nil
}

func (dao *DepartmentDAO) DeleteDepartment(ctx context.Context, departmentID string) error {
	start := time.Now()
	logger.Info("Deleting department", zap.String("deptID", departmentID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (d:DEPARTMENT {id: $id})
        DETACH DELETE d
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": departmentID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrDepartmentNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete department",
			zap.Error(err),
			zap.String("deptID", departmentID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Department deleted successfully",
		zap.String("deptID", departmentID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_DEPARTMENT",
		ResourceID:    departmentID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *DepartmentDAO) GetDepartment(ctx context.Context, departmentID string) (*model.Department, error) {
	start := time.Now()
	logger.Info("Retrieving department", zap.String("deptID", departmentID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (d:DEPARTMENT {id: $id})
    RETURN d
    `
	result, err := session.Run(query, map[string]interface{}{"id": departmentID})
	if err != nil {
		logger.Error("Failed to execute get department query",
			zap.Error(err),
			zap.String("deptID", departmentID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.String("deptID", departmentID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		logger.Info("Department retrieved successfully",
			zap.String("deptID", departmentID),
			zap.Duration("duration", time.Since(start)))
		return dept, nil
	}

	logger.Warn("Department not found",
		zap.String("deptID", departmentID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrDepartmentNotFound
}

func (dao *DepartmentDAO) ListDepartments(ctx context.Context, limit int, offset int) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Listing departments", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (d:DEPARTMENT)
    RETURN d
    ORDER BY d.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list departments query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var departments []*model.Department
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		departments = append(departments, dept)
	}

	logger.Info("Departments listed successfully",
		zap.Int("count", len(departments)),
		zap.Duration("duration", time.Since(start)))

	return departments, nil
}

// Helper function to map Neo4j Node to Department struct
func mapNodeToDepartment(node neo4j.Node) (*model.Department, error) {
	props := node.Props
	dept := &model.Department{}

	dept.ID = props["id"].(string)
	dept.Name = props["name"].(string)
	dept.OrganizationID = props["organizationID"].(string)
	if parentID, ok := props["parentID"].(string); ok {
		dept.ParentID = parentID
	}
	dept.CreatedAt = helper_util.ParseTime(props["createdAt"].(string))
	dept.UpdatedAt = helper_util.ParseTime(props["updatedAt"].(string))

	return dept, nil
}

// Helper function to create change details for audit log
func createDeptChangeDetails(oldDept, newDept *model.Department) json.RawMessage {
	changes := make(map[string]interface{})
	if oldDept == nil {
		changes["action"] = "created"
	} else if newDept == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldDept.Name != newDept.Name {
			changes["name"] = map[string]string{"old": oldDept.Name, "new": newDept.Name}
		}
		if oldDept.OrganizationID != newDept.OrganizationID {
			changes["organizationID"] = map[string]string{"old": oldDept.OrganizationID, "new": newDept.OrganizationID}
		}
		if oldDept.ParentID != newDept.ParentID {
			changes["parentID"] = map[string]string{"old": oldDept.ParentID, "new": newDept.ParentID}
		}
		// Add more fields as needed
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}

// Additional methods

// GetDepartmentsByOrganization retrieves all departments for a given organization
func (dao *DepartmentDAO) GetDepartmentsByOrganization(ctx context.Context, orgID string) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Retrieving departments by organization", zap.String("orgID", orgID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `MATCH (d:DEPARTMENT)-[:BELONGS_TO]->(o:ORGANIZATION {id: $orgId})
    RETURN d
    ORDER BY d.name
    `
	result, err := session.Run(query, map[string]interface{}{"orgId": orgID})
	if err != nil {
		logger.Error("Failed to execute get departments by organization query",
			zap.Error(err),
			zap.String("orgID", orgID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var departments []*model.Department
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		departments = append(departments, dept)
	}

	logger.Info("Departments retrieved successfully",
		zap.String("orgID", orgID),
		zap.Int("count", len(departments)),
		zap.Duration("duration", time.Since(start)))

	return departments, nil
}

// GetDepartmentHierarchy retrieves the department hierarchy for a given department
func (dao *DepartmentDAO) GetDepartmentHierarchy(ctx context.Context, deptID string) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Retrieving department hierarchy", zap.String("deptID", deptID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (d:DEPARTMENT {id: $deptId})
    MATCH (d)-[:BELONGS_TO*0..]->(parent:DEPARTMENT)
    RETURN parent
    ORDER BY length(((d)-[:BELONGS_TO*]->(parent))) DESC
    `
	result, err := session.Run(query, map[string]interface{}{"deptId": deptID})
	if err != nil {
		logger.Error("Failed to execute get department hierarchy query",
			zap.Error(err),
			zap.String("deptID", deptID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var hierarchy []*model.Department
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		hierarchy = append(hierarchy, dept)
	}

	logger.Info("Department hierarchy retrieved successfully",
		zap.String("deptID", deptID),
		zap.Int("hierarchyDepth", len(hierarchy)),
		zap.Duration("duration", time.Since(start)))

	return hierarchy, nil
}

// GetChildDepartments retrieves all immediate child departments of a given department
func (dao *DepartmentDAO) GetChildDepartments(ctx context.Context, parentDeptID string) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Retrieving child departments", zap.String("parentDeptID", parentDeptID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (parent:DEPARTMENT {id: $parentId})<-[:BELONGS_TO]-(child:DEPARTMENT)
    RETURN child
    ORDER BY child.name
    `
	result, err := session.Run(query, map[string]interface{}{"parentId": parentDeptID})
	if err != nil {
		logger.Error("Failed to execute get child departments query",
			zap.Error(err),
			zap.String("parentDeptID", parentDeptID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var childDepartments []*model.Department
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		childDepartments = append(childDepartments, dept)
	}

	logger.Info("Child departments retrieved successfully",
		zap.String("parentDeptID", parentDeptID),
		zap.Int("childCount", len(childDepartments)),
		zap.Duration("duration", time.Since(start)))

	return childDepartments, nil
}

// MoveDepartment moves a department to a new parent department
func (dao *DepartmentDAO) MoveDepartment(ctx context.Context, deptID string, newParentID string) error {
	start := time.Now()
	logger.Info("Moving department", zap.String("deptID", deptID), zap.String("newParentID", newParentID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (d:DEPARTMENT {id: $deptId})
        MATCH (newParent:DEPARTMENT {id: $newParentId})
        OPTIONAL MATCH (d)-[r:BELONGS_TO]->(:DEPARTMENT)
        DELETE r
        MERGE (d)-[:BELONGS_TO]->(newParent)
        SET d.parentID = $newParentId, d.updatedAt = $updatedAt
        RETURN d
        `
		params := map[string]interface{}{
			"deptId":      deptID,
			"newParentId": newParentID,
			"updatedAt":   time.Now().Format(time.RFC3339),
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if !result.Next() {
			return nil, echo_errors.ErrDepartmentNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to move department",
			zap.Error(err),
			zap.String("deptID", deptID),
			zap.String("newParentID", newParentID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Department moved successfully",
		zap.String("deptID", deptID),
		zap.String("newParentID", newParentID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "MOVE_DEPARTMENT",
		ResourceID:    deptID,
		AccessGranted: true,
		ChangeDetails: json.RawMessage(fmt.Sprintf(`{"newParentID": "%s"}`, newParentID)),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

// SearchDepartments searches for departments based on a name pattern
func (dao *DepartmentDAO) SearchDepartments(ctx context.Context, criteria model.DepartmentSearchCriteria) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Searching departments", zap.Any("criteria", criteria))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	var queryBuilder strings.Builder
	queryBuilder.WriteString("MATCH (d:DEPARTMENT) WHERE 1=1")

	params := make(map[string]interface{})

	if criteria.ID != "" {
		queryBuilder.WriteString(" AND d.id = $id")
		params["id"] = criteria.ID
	}

	if criteria.Name != "" {
		queryBuilder.WriteString(" AND toLower(d.name) CONTAINS toLower($name)")
		params["name"] = criteria.Name
	}

	if criteria.OrganizationID != "" {
		queryBuilder.WriteString(" AND d.organizationID = $orgID")
		params["orgID"] = criteria.OrganizationID
	}

	if criteria.ParentID != "" {
		queryBuilder.WriteString(" AND d.parentID = $parentID")
		params["parentID"] = criteria.ParentID
	}

	if criteria.FromDate != nil {
		queryBuilder.WriteString(" AND d.createdAt >= $fromDate")
		params["fromDate"] = criteria.FromDate.Format(time.RFC3339)
	}

	if criteria.ToDate != nil {
		queryBuilder.WriteString(" AND d.createdAt <= $toDate")
		params["toDate"] = criteria.ToDate.Format(time.RFC3339)
	}

	queryBuilder.WriteString(" RETURN d")

	if criteria.SortBy != "" {
		queryBuilder.WriteString(fmt.Sprintf(" ORDER BY d.%s", criteria.SortBy))
		if criteria.SortOrder != "" {
			queryBuilder.WriteString(" " + strings.ToUpper(criteria.SortOrder))
		} else {
			queryBuilder.WriteString(" ASC")
		}
	} else {
		queryBuilder.WriteString(" ORDER BY d.name ASC")
	}

	if criteria.Offset > 0 {
		queryBuilder.WriteString(" SKIP $offset")
		params["offset"] = criteria.Offset
	}

	if criteria.Limit > 0 {
		queryBuilder.WriteString(" LIMIT $limit")
		params["limit"] = criteria.Limit
	}

	logger.Info("Executing query", zap.String("query", queryBuilder.String()), zap.Any("params", params))

	result, err := session.Run(queryBuilder.String(), params)
	if err != nil {
		logger.Error("Failed to execute search departments query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var departments []*model.Department
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		dept, err := mapNodeToDepartment(node)
		if err != nil {
			logger.Error("Failed to map department node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		departments = append(departments, dept)
	}

	logger.Info("Departments searched successfully",
		zap.Int("resultCount", len(departments)),
		zap.Duration("duration", time.Since(start)))

	return departments, nil
}
