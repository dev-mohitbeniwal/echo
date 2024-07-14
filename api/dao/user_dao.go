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
        FOR (u:User) REQUIRE u.id IS UNIQUE
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
        MERGE (u:User {id: $id})
        ON CREATE SET u += $props
        ON MATCH SET u += $props
        RETURN u.id as id
        `

		attributesJSON, _ := json.Marshal(user.Attributes)

		params := map[string]interface{}{
			"id": user.ID,
			"props": map[string]interface{}{
				"username":       user.Username,
				"email":          user.Email,
				"userType":       user.UserType,
				"organizationID": user.OrganizationID,
				"departmentID":   user.DepartmentID,
				"attributes":     string(attributesJSON),
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

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string), // Assuming you store the requesting user's ID in the context
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
        MATCH (u:User {id: $id})
        SET u += $props
        RETURN u
        `

		attributesJSON, _ := json.Marshal(user.Attributes)

		params := map[string]interface{}{
			"id": user.ID,
			"props": map[string]interface{}{
				"username":       user.Username,
				"email":          user.Email,
				"userType":       user.UserType,
				"organizationID": user.OrganizationID,
				"departmentID":   user.DepartmentID,
				"attributes":     string(attributesJSON),
				"updatedAt":      time.Now().Format(time.RFC3339),
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
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
        MATCH (u:User {id: $id})
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
    MATCH (u:User {id: $id})
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
    MATCH (u:User)
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
