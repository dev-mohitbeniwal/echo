// api/service/role_service.go
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

// IRoleService defines the interface for role operations
type IRoleService interface {
	CreateRole(ctx context.Context, role model.Role, creatorID string) (*model.Role, error)
	UpdateRole(ctx context.Context, role model.Role, updaterID string) (*model.Role, error)
	DeleteRole(ctx context.Context, roleID string, deleterID string) error
	GetRole(ctx context.Context, roleID string) (*model.Role, error)
	ListRoles(ctx context.Context, limit int, offset int) ([]*model.Role, error)
	SearchRoles(ctx context.Context, query string, limit, offset int) ([]*model.Role, error)
}

// RoleService handles business logic for role operations
type RoleService struct {
	roleDAO         *dao.RoleDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IRoleService = &RoleService{}

// NewRoleService creates a new instance of RoleService
func NewRoleService(roleDAO *dao.RoleDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *RoleService {
	service := &RoleService{
		roleDAO:         roleDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("role.created", service.handleRoleCreated)
	eventBus.Subscribe("role.updated", service.handleRoleUpdated)
	eventBus.Subscribe("role.deleted", service.handleRoleDeleted)

	return service
}

func (s *RoleService) handleRoleCreated(ctx context.Context, event util.Event) error {
	role := event.Payload.(model.Role)
	logger.Info("Role created event received", zap.String("roleID", role.ID))

	// Update any indexes or materialized views
	if err := s.updateRoleIndexes(ctx, role); err != nil {
		logger.Error("Failed to update role indexes", zap.Error(err), zap.String("roleID", role.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyRoleChange(ctx, "created", role); err != nil {
		logger.Warn("Failed to send role creation notification", zap.Error(err), zap.String("roleID", role.ID))
	}

	return nil
}

func (s *RoleService) handleRoleUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Role)
	oldRole, newRole := payload["old"], payload["new"]

	logger.Info("Role updated event received",
		zap.String("roleID", newRole.ID),
		zap.Time("oldUpdatedAt", oldRole.UpdatedAt),
		zap.Time("newUpdatedAt", newRole.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateRoleIndexes(ctx, newRole); err != nil {
		logger.Error("Failed to update role indexes", zap.Error(err), zap.String("roleID", newRole.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyRoleChange(ctx, "updated", newRole); err != nil {
		logger.Warn("Failed to send role update notification", zap.Error(err), zap.String("roleID", newRole.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this role change
	if err := s.invalidateRelatedCaches(ctx, newRole.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("roleID", newRole.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *RoleService) handleRoleDeleted(ctx context.Context, event util.Event) error {
	roleID := event.Payload.(string)
	logger.Info("Role deleted event received", zap.String("roleID", roleID))

	// Remove role from any indexes or materialized views
	if err := s.removeRoleFromIndexes(ctx, roleID); err != nil {
		logger.Error("Failed to remove role from indexes", zap.Error(err), zap.String("roleID", roleID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyRoleChange(ctx, "deleted", model.Role{ID: roleID}); err != nil {
		logger.Warn("Failed to send role deletion notification", zap.Error(err), zap.String("roleID", roleID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupRoleRelatedData(ctx, roleID); err != nil {
		logger.Error("Failed to clean up role-related data", zap.Error(err), zap.String("roleID", roleID))
		// Continue execution despite the error
	}

	return nil
}

// CreateRole handles the creation of a new role
func (s *RoleService) CreateRole(ctx context.Context, role model.Role, creatorID string) (*model.Role, error) {
	if err := s.validationUtil.ValidateRole(role); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	roleID, err := s.roleDAO.CreateRole(ctx, role)
	if err != nil {
		logger.Error("Error creating role", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	role.ID = roleID

	// Update cache
	if err := s.cacheService.SetRole(ctx, role); err != nil {
		logger.Warn("Failed to cache role", zap.Error(err), zap.String("roleID", roleID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "role.created", role)

	logger.Info("Role created successfully", zap.String("roleID", roleID), zap.String("creatorID", creatorID))
	return &role, nil
}

// UpdateRole handles updates to an existing role
func (s *RoleService) UpdateRole(ctx context.Context, role model.Role, updaterID string) (*model.Role, error) {
	if err := s.validationUtil.ValidateRole(role); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	oldRole, err := s.roleDAO.GetRole(ctx, role.ID)
	if err != nil {
		logger.Error("Error retrieving existing role", zap.Error(err), zap.String("roleID", role.ID))
		return nil, err
	}

	role.UpdatedAt = time.Now()

	updatedRole, err := s.roleDAO.UpdateRole(ctx, role)
	if err != nil {
		logger.Error("Error updating role", zap.Error(err), zap.String("roleID", role.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetRole(ctx, *updatedRole); err != nil {
		logger.Warn("Failed to update role in cache", zap.Error(err), zap.String("roleID", role.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "role.updated", map[string]model.Role{
		"old": *oldRole,
		"new": *updatedRole,
	})

	logger.Info("Role updated successfully", zap.String("roleID", role.ID), zap.String("updaterID", updaterID))
	return updatedRole, nil
}

// DeleteRole handles the deletion of a role
func (s *RoleService) DeleteRole(ctx context.Context, roleID string, deleterID string) error {
	err := s.roleDAO.DeleteRole(ctx, roleID)
	if err != nil {
		logger.Error("Error deleting role", zap.Error(err), zap.String("roleID", roleID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete role: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteRole(ctx, roleID); err != nil {
		logger.Warn("Failed to delete role from cache", zap.Error(err), zap.String("roleID", roleID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "role.deleted", roleID)

	logger.Info("Role deleted successfully", zap.String("roleID", roleID), zap.String("deleterID", deleterID))
	return nil
}

// GetRole retrieves a role by its ID
func (s *RoleService) GetRole(ctx context.Context, roleID string) (*model.Role, error) {
	// Try to get from cache first
	cachedRole, err := s.cacheService.GetRole(ctx, roleID)
	if err == nil && cachedRole != nil {
		return cachedRole, nil
	}

	role, err := s.roleDAO.GetRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrRoleNotFound) {
			return nil, echo_errors.ErrRoleNotFound
		}
		logger.Error("Error retrieving role", zap.Error(err), zap.String("roleID", roleID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetRole(ctx, *role); err != nil {
		logger.Warn("Failed to cache role", zap.Error(err), zap.String("roleID", roleID))
	}

	return role, nil
}

// ListRoles retrieves all roles, possibly with pagination
func (s *RoleService) ListRoles(ctx context.Context, limit int, offset int) ([]*model.Role, error) {
	roles, err := s.roleDAO.ListRoles(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing roles", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, nil
}

// SearchRoles searches for roles based on a query string
func (s *RoleService) SearchRoles(ctx context.Context, query string, limit, offset int) ([]*model.Role, error) {
	// Implement role search logic here
	// This might involve searching by role name, description, or other attributes
	// You may need to add a corresponding method in the RoleDAO
	return nil, fmt.Errorf("role search not implemented")
}

// Helper methods

func (s *RoleService) updateRoleIndexes(ctx context.Context, role model.Role) error {
	// Implementation for updating indexes
	return nil
}

func (s *RoleService) invalidateRelatedCaches(ctx context.Context, roleID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *RoleService) removeRoleFromIndexes(ctx context.Context, roleID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *RoleService) cleanupRoleRelatedData(ctx context.Context, roleID string) error {
	// Implementation for cleaning up related data
	return nil
}
