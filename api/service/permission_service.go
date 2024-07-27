// api/service/permission_service.go
package service

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/dao"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

// IPermissionService defines the interface for permission operations
type IPermissionService interface {
	CreatePermission(ctx context.Context, permission model.Permission, creatorID string) (*model.Permission, error)
	UpdatePermission(ctx context.Context, permission model.Permission, updaterID string) (*model.Permission, error)
	DeletePermission(ctx context.Context, permissionID string, deleterID string) error
	GetPermission(ctx context.Context, permissionID string) (*model.Permission, error)
	ListPermissions(ctx context.Context, limit int, offset int) ([]*model.Permission, error)
	SearchPermissions(ctx context.Context, query string, limit, offset int) ([]*model.Permission, error)
}

// PermissionService handles business logic for permission operations
type PermissionService struct {
	permissionDAO   *dao.PermissionDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IPermissionService = &PermissionService{}

// NewPermissionService creates a new instance of PermissionService
func NewPermissionService(permissionDAO *dao.PermissionDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *PermissionService {
	service := &PermissionService{
		permissionDAO:   permissionDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("permission.created", service.handlePermissionCreated)
	eventBus.Subscribe("permission.updated", service.handlePermissionUpdated)
	eventBus.Subscribe("permission.deleted", service.handlePermissionDeleted)

	return service
}

func (s *PermissionService) handlePermissionCreated(ctx context.Context, event util.Event) error {
	permission := event.Payload.(model.Permission)
	logger.Info("Permission created event received", zap.String("permissionID", permission.ID))

	// Update any indexes or materialized views
	if err := s.updatePermissionIndexes(ctx, permission); err != nil {
		logger.Error("Failed to update permission indexes", zap.Error(err), zap.String("permissionID", permission.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPermissionChange(ctx, "created", permission); err != nil {
		logger.Warn("Failed to send permission creation notification", zap.Error(err), zap.String("permissionID", permission.ID))
	}

	return nil
}

func (s *PermissionService) handlePermissionUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Permission)
	_, newPermission := payload["old"], payload["new"]

	logger.Info("Permission updated event received",
		zap.String("permissionID", newPermission.ID))

	// Update any indexes or materialized views
	if err := s.updatePermissionIndexes(ctx, newPermission); err != nil {
		logger.Error("Failed to update permission indexes", zap.Error(err), zap.String("permissionID", newPermission.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPermissionChange(ctx, "updated", newPermission); err != nil {
		logger.Warn("Failed to send permission update notification", zap.Error(err), zap.String("permissionID", newPermission.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this permission change
	if err := s.invalidateRelatedCaches(ctx, newPermission.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("permissionID", newPermission.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *PermissionService) handlePermissionDeleted(ctx context.Context, event util.Event) error {
	permissionID := event.Payload.(string)
	logger.Info("Permission deleted event received", zap.String("permissionID", permissionID))

	// Remove permission from any indexes or materialized views
	if err := s.removePermissionFromIndexes(ctx, permissionID); err != nil {
		logger.Error("Failed to remove permission from indexes", zap.Error(err), zap.String("permissionID", permissionID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyPermissionChange(ctx, "deleted", model.Permission{ID: permissionID}); err != nil {
		logger.Warn("Failed to send permission deletion notification", zap.Error(err), zap.String("permissionID", permissionID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupPermissionRelatedData(ctx, permissionID); err != nil {
		logger.Error("Failed to clean up permission-related data", zap.Error(err), zap.String("permissionID", permissionID))
		// Continue execution despite the error
	}

	return nil
}

// CreatePermission handles the creation of a new permission
func (s *PermissionService) CreatePermission(ctx context.Context, permission model.Permission, creatorID string) (*model.Permission, error) {
	if err := s.validationUtil.ValidatePermission(permission); err != nil {
		return nil, fmt.Errorf("invalid permission: %w", err)
	}

	permissionID, err := s.permissionDAO.CreatePermission(ctx, permission)
	if err != nil {
		logger.Error("Error creating permission", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	permission.ID = permissionID

	// Update cache
	if err := s.cacheService.SetPermission(ctx, permission); err != nil {
		logger.Warn("Failed to cache permission", zap.Error(err), zap.String("permissionID", permissionID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "permission.created", permission)

	logger.Info("Permission created successfully", zap.String("permissionID", permissionID), zap.String("creatorID", creatorID))
	return &permission, nil
}

// UpdatePermission handles updates to an existing permission
func (s *PermissionService) UpdatePermission(ctx context.Context, permission model.Permission, updaterID string) (*model.Permission, error) {
	if err := s.validationUtil.ValidatePermission(permission); err != nil {
		return nil, fmt.Errorf("invalid permission: %w", err)
	}

	oldPermission, err := s.permissionDAO.GetPermission(ctx, permission.ID)
	if err != nil {
		logger.Error("Error retrieving existing permission", zap.Error(err), zap.String("permissionID", permission.ID))
		return nil, err
	}

	updatedPermission, err := s.permissionDAO.UpdatePermission(ctx, permission)
	if err != nil {
		logger.Error("Error updating permission", zap.Error(err), zap.String("permissionID", permission.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update permission: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetPermission(ctx, *updatedPermission); err != nil {
		logger.Warn("Failed to update permission in cache", zap.Error(err), zap.String("permissionID", permission.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "permission.updated", map[string]model.Permission{
		"old": *oldPermission,
		"new": *updatedPermission,
	})

	logger.Info("Permission updated successfully", zap.String("permissionID", permission.ID), zap.String("updaterID", updaterID))
	return updatedPermission, nil
}

// DeletePermission handles the deletion of a permission
func (s *PermissionService) DeletePermission(ctx context.Context, permissionID string, deleterID string) error {
	err := s.permissionDAO.DeletePermission(ctx, permissionID)
	if err != nil {
		logger.Error("Error deleting permission", zap.Error(err), zap.String("permissionID", permissionID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeletePermission(ctx, permissionID); err != nil {
		logger.Warn("Failed to delete permission from cache", zap.Error(err), zap.String("permissionID", permissionID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "permission.deleted", permissionID)

	logger.Info("Permission deleted successfully", zap.String("permissionID", permissionID), zap.String("deleterID", deleterID))
	return nil
}

// GetPermission retrieves a permission by its ID
func (s *PermissionService) GetPermission(ctx context.Context, permissionID string) (*model.Permission, error) {
	// Try to get from cache first
	cachedPermission, err := s.cacheService.GetPermission(ctx, permissionID)
	if err == nil && cachedPermission != nil {
		return cachedPermission, nil
	}

	permission, err := s.permissionDAO.GetPermission(ctx, permissionID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrPermissionNotFound) {
			return nil, echo_errors.ErrPermissionNotFound
		}
		logger.Error("Error retrieving permission", zap.Error(err), zap.String("permissionID", permissionID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetPermission(ctx, *permission); err != nil {
		logger.Warn("Failed to cache permission", zap.Error(err), zap.String("permissionID", permissionID))
	}

	return permission, nil
}

// ListPermissions retrieves all permissions, possibly with pagination
func (s *PermissionService) ListPermissions(ctx context.Context, limit int, offset int) ([]*model.Permission, error) {
	permissions, err := s.permissionDAO.ListPermissions(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing permissions", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

// SearchPermissions searches for permissions based on a query string
func (s *PermissionService) SearchPermissions(ctx context.Context, query string, limit, offset int) ([]*model.Permission, error) {
	// Implement permission search logic here
	// This might involve searching by permission name, description, or action
	// You may need to add a corresponding method in the PermissionDAO
	return nil, fmt.Errorf("permission search not implemented")
}

// Helper methods

func (s *PermissionService) updatePermissionIndexes(ctx context.Context, permission model.Permission) error {
	// Implementation for updating indexes
	return nil
}

func (s *PermissionService) invalidateRelatedCaches(ctx context.Context, permissionID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *PermissionService) removePermissionFromIndexes(ctx context.Context, permissionID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *PermissionService) cleanupPermissionRelatedData(ctx context.Context, permissionID string) error {
	// Implementation for cleaning up related data
	return nil
}
