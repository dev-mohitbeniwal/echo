// api/service/user_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/dao"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

// IUserService defines the interface for user operations
type IUserService interface {
	CreateUser(ctx context.Context, user model.User, creatorID string) (*model.User, error)
	UpdateUser(ctx context.Context, user model.User, updaterID string) (*model.User, error)
	DeleteUser(ctx context.Context, userID string, deleterID string) error
	GetUser(ctx context.Context, userID string) (*model.User, error)
	ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error)
	SearchUsers(ctx context.Context, query string, limit, offset int) ([]*model.User, error)
}

// UserService handles business logic for user operations
type UserService struct {
	userDAO         *dao.UserDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IUserService = &UserService{}

// NewUserService creates a new instance of UserService
func NewUserService(userDAO *dao.UserDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *UserService {
	service := &UserService{
		userDAO:         userDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("user.created", service.handleUserCreated)
	eventBus.Subscribe("user.updated", service.handleUserUpdated)
	eventBus.Subscribe("user.deleted", service.handleUserDeleted)

	return service
}

func (s *UserService) handleUserCreated(ctx context.Context, event util.Event) error {
	user := event.Payload.(model.User)
	logger.Info("User created event received", zap.String("userID", user.ID))

	// Update any indexes or materialized views
	if err := s.updateUserIndexes(ctx, user); err != nil {
		logger.Error("Failed to update user indexes", zap.Error(err), zap.String("userID", user.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyUserChange(ctx, "created", user); err != nil {
		logger.Warn("Failed to send user creation notification", zap.Error(err), zap.String("userID", user.ID))
	}

	return nil
}

func (s *UserService) handleUserUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.User)
	oldUser, newUser := payload["old"], payload["new"]

	logger.Info("User updated event received",
		zap.String("userID", newUser.ID),
		zap.Time("oldUpdatedAt", oldUser.UpdatedAt),
		zap.Time("newUpdatedAt", newUser.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateUserIndexes(ctx, newUser); err != nil {
		logger.Error("Failed to update user indexes", zap.Error(err), zap.String("userID", newUser.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyUserChange(ctx, "updated", newUser); err != nil {
		logger.Warn("Failed to send user update notification", zap.Error(err), zap.String("userID", newUser.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this user change
	if err := s.invalidateRelatedCaches(ctx, newUser.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("userID", newUser.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *UserService) handleUserDeleted(ctx context.Context, event util.Event) error {
	userID := event.Payload.(string)
	logger.Info("User deleted event received", zap.String("userID", userID))

	// Remove user from any indexes or materialized views
	if err := s.removeUserFromIndexes(ctx, userID); err != nil {
		logger.Error("Failed to remove user from indexes", zap.Error(err), zap.String("userID", userID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyUserChange(ctx, "deleted", model.User{ID: userID}); err != nil {
		logger.Warn("Failed to send user deletion notification", zap.Error(err), zap.String("userID", userID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupUserRelatedData(ctx, userID); err != nil {
		logger.Error("Failed to clean up user-related data", zap.Error(err), zap.String("userID", userID))
		// Continue execution despite the error
	}

	return nil
}

// CreateUser handles the creation of a new user
func (s *UserService) CreateUser(ctx context.Context, user model.User, creatorID string) (*model.User, error) {
	if err := s.validationUtil.ValidateUser(user); err != nil {
		return nil, fmt.Errorf("invalid user: %w", err)
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	userID, err := s.userDAO.CreateUser(ctx, user)
	if err != nil {
		logger.Error("Error creating user", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	user.ID = userID

	// Update cache
	if err := s.cacheService.SetUser(ctx, user); err != nil {
		logger.Warn("Failed to cache user", zap.Error(err), zap.String("userID", userID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "user.created", user)

	logger.Info("User created successfully", zap.String("userID", userID), zap.String("creatorID", creatorID))
	return &user, nil
}

// UpdateUser handles updates to an existing user
func (s *UserService) UpdateUser(ctx context.Context, user model.User, updaterID string) (*model.User, error) {
	if err := s.validationUtil.ValidateUser(user); err != nil {
		return nil, fmt.Errorf("invalid user: %w", err)
	}

	oldUser, err := s.userDAO.GetUser(ctx, user.ID)
	if err != nil {
		logger.Error("Error retrieving existing user", zap.Error(err), zap.String("userID", user.ID))
		return nil, err
	}

	user.UpdatedAt = time.Now()

	updatedUser, err := s.userDAO.UpdateUser(ctx, user)
	if err != nil {
		logger.Error("Error updating user", zap.Error(err), zap.String("userID", user.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetUser(ctx, *updatedUser); err != nil {
		logger.Warn("Failed to update user in cache", zap.Error(err), zap.String("userID", user.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "user.updated", map[string]model.User{
		"old": *oldUser,
		"new": *updatedUser,
	})

	logger.Info("User updated successfully", zap.String("userID", user.ID), zap.String("updaterID", updaterID))
	return updatedUser, nil
}

// DeleteUser handles the deletion of a user
func (s *UserService) DeleteUser(ctx context.Context, userID string, deleterID string) error {
	err := s.userDAO.DeleteUser(ctx, userID)
	if err != nil {
		logger.Error("Error deleting user", zap.Error(err), zap.String("userID", userID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteUser(ctx, userID); err != nil {
		logger.Warn("Failed to delete user from cache", zap.Error(err), zap.String("userID", userID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "user.deleted", userID)

	logger.Info("User deleted successfully", zap.String("userID", userID), zap.String("deleterID", deleterID))
	return nil
}

// GetUser retrieves a user by their ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	// Try to get from cache first
	cachedUser, err := s.cacheService.GetUser(ctx, userID)
	if err == nil && cachedUser != nil {
		return cachedUser, nil
	}

	user, err := s.userDAO.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrUserNotFound) {
			return nil, echo_errors.ErrUserNotFound
		}
		logger.Error("Error retrieving user", zap.Error(err), zap.String("userID", userID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetUser(ctx, *user); err != nil {
		logger.Warn("Failed to cache user", zap.Error(err), zap.String("userID", userID))
	}

	return user, nil
}

// ListUsers retrieves all users, possibly with pagination
func (s *UserService) ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error) {
	users, err := s.userDAO.ListUsers(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing users", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// SearchUsers searches for users based on a query string
func (s *UserService) SearchUsers(ctx context.Context, query string, limit, offset int) ([]*model.User, error) {
	// Implement user search logic here
	// This might involve searching by username, email, or other attributes
	// You may need to add a corresponding method in the UserDAO
	return nil, fmt.Errorf("user search not implemented")
}

// Helper methods

func (s *UserService) updateUserIndexes(ctx context.Context, user model.User) error {
	// Implementation for updating indexes
	return nil
}

func (s *UserService) invalidateRelatedCaches(ctx context.Context, userID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *UserService) removeUserFromIndexes(ctx context.Context, userID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *UserService) cleanupUserRelatedData(ctx context.Context, userID string) error {
	// Implementation for cleaning up related data
	return nil
}
