// api/dao/user_dao.go
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

type UserDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewUserDAO(driver neo4j.Driver, auditService audit.Service) *UserDAO {
	dao := &UserDAO{Driver: driver, AuditService: auditService}
	// Ensure unique constraint on User ID
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for User", zap.Error(err))
	}
	return dao
}

func (dao *UserDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on User ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_user_id IF NOT EXISTS
        FOR (u:` + echo_neo4j.LabelUser + `) REQUIRE u.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on User ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on User ID")
	return nil
}

func (dao *UserDAO) CreateUser(ctx context.Context, user model.User) (string, error) {
	start := time.Now()
	logger.Info("Creating new user", zap.String("username", user.Username))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
            CREATE (u:USER {id: $id})
            SET u += $props
            WITH u
            OPTIONAL MATCH (o:ORGANIZATION {id: $organizationID})
            FOREACH (_ IN CASE WHEN o IS NOT NULL THEN [1] ELSE [] END |
                CREATE (u)-[:WORKS_FOR]->(o)
            )
            WITH u
            OPTIONAL MATCH (d:DEPARTMENT {id: $departmentID})
            FOREACH (_ IN CASE WHEN d IS NOT NULL THEN [1] ELSE [] END |
                CREATE (u)-[:MEMBER_OF]->(d)
            )
        `

		// Conditionally add role relationships
		if len(user.RoleIds) > 0 {
			query += `
                WITH u
                UNWIND $roleIds AS roleId
                OPTIONAL MATCH (r:ROLE {id: roleId})
                FOREACH (_ IN CASE WHEN r IS NOT NULL THEN [1] ELSE [] END |
                    CREATE (u)-[:HAS_ROLE]->(r)
                )
            `
		}

		// Conditionally add group relationships
		if len(user.GroupIds) > 0 {
			query += `
                WITH u
                UNWIND $groupIds AS groupId
                OPTIONAL MATCH (g:GROUP {id: groupId})
                FOREACH (_ IN CASE WHEN g IS NOT NULL THEN [1] ELSE [] END |
                    CREATE (u)-[:BELONGS_TO_GROUP]->(g)
                )
            `
		}

		query += `
            RETURN u.id as id, u.name as name, u.email as email
        `

		attributesJSON, err := json.Marshal(user.Attributes)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attributes: %w", err)
		}

		now := time.Now().Format(time.RFC3339)
		params := map[string]interface{}{
			"id": user.ID,
			"props": map[string]interface{}{
				"name":           user.Name,
				"username":       user.Username,
				"email":          user.Email,
				"userType":       user.UserType,
				"organizationID": user.OrganizationID,
				"departmentID":   user.DepartmentID,
				"attributes":     string(attributesJSON),
				"status":         user.Status,
				"createdAt":      now,
				"updatedAt":      now,
			},
			"organizationID": user.OrganizationID,
			"departmentID":   user.DepartmentID,
			"roleIds":        user.RoleIds,
			"groupIds":       user.GroupIds,
		}

		// Handle nil slices
		if user.RoleIds == nil {
			params["roleIds"] = []string{}
		}
		if user.GroupIds == nil {
			params["groupIds"] = []string{}
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
		logger.Error("Failed to create user",
			zap.Error(err),
			zap.String("username", user.Username),
			zap.Duration("duration", duration))
		return "", err
	}

	userID := fmt.Sprintf("%v", result)
	logger.Info("User created successfully",
		zap.String("userID", userID),
		zap.Duration("duration", duration))

	// Verify relationships
	verifyErr := dao.verifyRelationships(ctx, userID, user.OrganizationID, user.DepartmentID)
	if verifyErr != nil {
		logger.Error("Failed to verify relationships",
			zap.Error(verifyErr),
			zap.String("userID", userID),
			zap.String("orgID", user.OrganizationID),
			zap.String("deptID", user.DepartmentID))
	}

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_USER",
		ResourceID:    userID,
		AccessGranted: true,
		ChangeDetails: createUserChangeDetails(nil, &user),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return userID, nil
}

func (dao *UserDAO) verifyRelationships(ctx context.Context, userID, orgID, deptID string) error {
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (u:` + echo_neo4j.LabelUser + ` {id: $userID})
    OPTIONAL MATCH (u)-[r:` + echo_neo4j.RelWorksFor + `]->(o:` + echo_neo4j.LabelOrganization + `)
    OPTIONAL MATCH (u)-[m:` + echo_neo4j.RelMemberOf + `]->(d:` + echo_neo4j.LabelDepartment + `)
    RETURN r, m, o.id as orgId, d.id as deptId
    `

	result, err := session.Run(query, map[string]interface{}{
		"userID": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to verify relationships: %w", err)
	}

	if result.Next() {
		record := result.Record()
		orgRel, _ := record.Get("r")
		deptRel, _ := record.Get("m")
		returnedOrgID, _ := record.Get("orgId")
		returnedDeptID, _ := record.Get("deptId")

		if orgID != "" && orgRel == nil {
			logger.Error("BELONGS_TO relationship not created", zap.String("userID", userID), zap.String("orgID", orgID))
		} else if orgID != "" {
			logger.Info("BELONGS_TO relationship verified", zap.String("userID", userID), zap.String("orgID", returnedOrgID.(string)))
		}

		if deptID != "" && deptRel == nil {
			logger.Error("MEMBER_OF relationship not created", zap.String("userID", userID), zap.String("deptID", deptID))
		} else if deptID != "" {
			logger.Info("MEMBER_OF relationship verified", zap.String("userID", userID), zap.String("deptID", returnedDeptID.(string)))
		}
	} else {
		return fmt.Errorf("no results returned when verifying relationships")
	}

	return nil
}

func (dao *UserDAO) UpdateUser(ctx context.Context, user model.User) (*model.User, error) {
	start := time.Now()
	logger.Info("Updating user", zap.String("userID", user.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedUser *model.User
	oldUser, err := dao.GetUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (u:` + echo_neo4j.LabelUser + ` {id: $id})
        SET u += $props
        WITH u
        OPTIONAL MATCH (u)-[oldOrgRel:` + echo_neo4j.RelWorksFor + `]->(:` + echo_neo4j.LabelOrganization + `)
        DELETE oldOrgRel
        WITH u
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $organizationID})
        MERGE (u)-[:` + echo_neo4j.RelWorksFor + `]->(o)
        WITH u
        OPTIONAL MATCH (u)-[oldDeptRel:` + echo_neo4j.RelMemberOf + `]->(:` + echo_neo4j.LabelDepartment + `)
        DELETE oldDeptRel
        WITH u
        OPTIONAL MATCH (d:` + echo_neo4j.LabelDepartment + ` {id: $departmentID})
        FOREACH (_ IN CASE WHEN d IS NOT NULL THEN [1] ELSE [] END |
            MERGE (u)-[:` + echo_neo4j.RelMemberOf + `]->(d)
        )
        WITH u
        OPTIONAL MATCH (u)-[oldRoleRel:` + echo_neo4j.RelHasRole + `]->(:` + echo_neo4j.LabelRole + `)
        DELETE oldRoleRel
        WITH u
        UNWIND $roleIds AS roleId
        MATCH (r:` + echo_neo4j.LabelRole + ` {id: roleId})
        MERGE (u)-[:` + echo_neo4j.RelHasRole + `]->(r)
        WITH u
        OPTIONAL MATCH (u)-[oldGroupRel:` + echo_neo4j.RelBelongsToGroup + `]->(:` + echo_neo4j.LabelGroup + `)
        DELETE oldGroupRel
        WITH u
        UNWIND $groupIds AS groupId
        MATCH (g:` + echo_neo4j.LabelGroup + ` {id: groupId})
        MERGE (u)-[:` + echo_neo4j.RelBelongsToGroup + `]->(g)
        RETURN u
        `

		attributesJSON, _ := json.Marshal(user.Attributes)

		params := map[string]interface{}{
			"id": user.ID,
			"props": map[string]interface{}{
				echo_neo4j.AttrName:      user.Name,
				"username":               user.Username,
				echo_neo4j.AttrEmail:     user.Email,
				echo_neo4j.AttrUserType:  user.UserType,
				"organizationID":         user.OrganizationID,
				"departmentID":           user.DepartmentID,
				"attributes":             string(attributesJSON),
				echo_neo4j.AttrUpdatedAt: time.Now().Format(time.RFC3339),
			},
			"organizationID": user.OrganizationID,
			"departmentID":   user.DepartmentID,
			"roleIds":        user.RoleIds,
			"groupIds":       user.GroupIds,
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			logger.Error("Failed to execute query", zap.Error(err), zap.Any("params", params))
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedUser, err = mapNodeToUser(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map user node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrUserNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update user",
			zap.Error(err),
			zap.String("userID", user.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("User updated successfully",
		zap.String("userID", user.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_USER",
		ResourceID:    user.ID,
		AccessGranted: true,
		ChangeDetails: createUserChangeDetails(oldUser, updatedUser),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedUser, nil
}

func (dao *UserDAO) DeleteUser(ctx context.Context, userID string) error {
	start := time.Now()
	logger.Info("Deleting user", zap.String("userID", userID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (u:` + echo_neo4j.LabelUser + ` {id: $id})
        DETACH DELETE u
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": userID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrUserNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete user",
			zap.Error(err),
			zap.String("userID", userID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("User deleted successfully",
		zap.String("userID", userID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_USER",
		ResourceID:    userID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *UserDAO) GetUser(ctx context.Context, userID string) (*model.User, error) {
	start := time.Now()
	logger.Info("Retrieving user", zap.String("userID", userID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (u:` + echo_neo4j.LabelUser + ` {id: $id})
    RETURN u
    `
	result, err := session.Run(query, map[string]interface{}{"id": userID})
	if err != nil {
		logger.Error("Failed to execute get user query",
			zap.Error(err),
			zap.String("userID", userID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		user, err := mapNodeToUser(node)
		if err != nil {
			logger.Error("Failed to map user node to struct",
				zap.Error(err),
				zap.String("userID", userID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		logger.Info("User retrieved successfully",
			zap.String("userID", userID),
			zap.Duration("duration", time.Since(start)))
		return user, nil
	}

	logger.Warn("User not found",
		zap.String("userID", userID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrUserNotFound
}

func (dao *UserDAO) ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error) {
	start := time.Now()
	logger.Info("Listing users", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (u:` + echo_neo4j.LabelUser + `)
    RETURN u
    ORDER BY u.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list users query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var users []*model.User
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		user, err := mapNodeToUser(node)
		if err != nil {
			logger.Error("Failed to map user node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		users = append(users, user)
	}

	logger.Info("Users listed successfully",
		zap.Int("count", len(users)),
		zap.Duration("duration", time.Since(start)))

	return users, nil
}

// Helper function to map Neo4j Node to User struct
func mapNodeToUser(node neo4j.Node) (*model.User, error) {
	props := node.Props
	user := &model.User{}

	user.ID = props["id"].(string)
	user.Username = props["username"].(string)
	user.Email = props["email"].(string)
	user.UserType = props["userType"].(string)
	user.OrganizationID = props["organizationID"].(string)
	user.DepartmentID = props["departmentID"].(string)

	attributesJSON := props["attributes"].(string)
	if err := json.Unmarshal([]byte(attributesJSON), &user.Attributes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user attributes: %w", err)
	}

	user.CreatedAt = helper_util.ParseTime(props["createdAt"].(string))
	user.UpdatedAt = helper_util.ParseTime(props["updatedAt"].(string))

	return user, nil
}

// Helper function to create change details for audit log
func createUserChangeDetails(oldUser, newUser *model.User) json.RawMessage {
	changes := make(map[string]interface{})
	if oldUser == nil {
		changes["action"] = "created"
	} else if newUser == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldUser.Username != newUser.Username {
			changes["username"] = map[string]string{"old": oldUser.Username, "new": newUser.Username}
		}
		if oldUser.Email != newUser.Email {
			changes["email"] = map[string]string{"old": oldUser.Email, "new": newUser.Email}
		}
		// Add more fields as needed
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}
