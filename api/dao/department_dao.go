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
        FOR (d:Department) REQUIRE d.id IS UNIQUE
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
        MERGE (d:Department {id: $id})
        ON CREATE SET d += $props
        WITH d
        MATCH (o:Organization {id: $orgId})
        MERGE (d)-[:BELONGS_TO]->(o)
        RETURN d.id as id
        `

		params := map[string]interface{}{
			"id":    department.ID,
			"orgId": department.OrganizationID,
			"props": map[string]interface{}{
				"name":           department.Name,
				"organizationID": department.OrganizationID,
				"parentID":       department.ParentID,
				"createdAt":      time.Now().Format(time.RFC3339),
				"updatedAt":      time.Now().Format(time.RFC3339),
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
        MATCH (d:Department {id: $id})
        SET d += $props
        WITH d
        MATCH (o:Organization {id: $orgId})
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
        MATCH (d:Department {id: $id})
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
    MATCH (d:Department {id: $id})
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
    MATCH (d:Department)
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

	query := `MATCH (d:Department)-[:BELONGS_TO]->(o:Organization {id: $orgId})
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
    MATCH (d:Department {id: $deptId})
    MATCH (d)-[:BELONGS_TO*0..]->(parent:Department)
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
    MATCH (parent:Department {id: $parentId})<-[:BELONGS_TO]-(child:Department)
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
        MATCH (d:Department {id: $deptId})
        MATCH (newParent:Department {id: $newParentId})
        OPTIONAL MATCH (d)-[r:BELONGS_TO]->(:Department)
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
func (dao *DepartmentDAO) SearchDepartments(ctx context.Context, namePattern string, limit int) ([]*model.Department, error) {
	start := time.Now()
	logger.Info("Searching departments", zap.String("namePattern", namePattern), zap.Int("limit", limit))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (d:Department)
    WHERE d.name =~ $namePattern
    RETURN d
    ORDER BY d.name
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"namePattern": "(?i).*" + namePattern + ".*",
		"limit":       limit,
	})
	if err != nil {
		logger.Error("Failed to execute search departments query",
			zap.Error(err),
			zap.String("namePattern", namePattern),
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
		zap.String("namePattern", namePattern),
		zap.Int("resultCount", len(departments)),
		zap.Duration("duration", time.Since(start)))

	return departments, nil
}
