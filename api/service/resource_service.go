// api/service/resource_service.go
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

// IResourceService defines the interface for resource operations
type IResourceService interface {
	CreateResource(ctx context.Context, resource model.Resource, creatorID string) (*model.Resource, error)
	UpdateResource(ctx context.Context, resource model.Resource, updaterID string) (*model.Resource, error)
	DeleteResource(ctx context.Context, resourceID string, deleterID string) error
	GetResource(ctx context.Context, resourceID string) (*model.Resource, error)
	ListResources(ctx context.Context, limit int, offset int) ([]*model.Resource, error)
	SearchResources(ctx context.Context, criteria model.ResourceSearchCriteria) ([]*model.Resource, error)
}

// ResourceService handles business logic for resource operations
type ResourceService struct {
	resourceDAO     *dao.ResourceDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IResourceService = &ResourceService{}

// NewResourceService creates a new instance of ResourceService
func NewResourceService(resourceDAO *dao.ResourceDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *ResourceService {
	service := &ResourceService{
		resourceDAO:     resourceDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("resource.created", service.handleResourceCreated)
	eventBus.Subscribe("resource.updated", service.handleResourceUpdated)
	eventBus.Subscribe("resource.deleted", service.handleResourceDeleted)

	return service
}

func (s *ResourceService) handleResourceCreated(ctx context.Context, event util.Event) error {
	resource := event.Payload.(model.Resource)
	logger.Info("Resource created event received", zap.String("resourceID", resource.ID))

	// Update any indexes or materialized views
	if err := s.updateResourceIndexes(ctx, resource); err != nil {
		logger.Error("Failed to update resource indexes", zap.Error(err), zap.String("resourceID", resource.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceChange(ctx, "created", resource); err != nil {
		logger.Warn("Failed to send resource creation notification", zap.Error(err), zap.String("resourceID", resource.ID))
	}

	return nil
}

func (s *ResourceService) handleResourceUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Resource)
	oldResource, newResource := payload["old"], payload["new"]

	logger.Info("Resource updated event received",
		zap.String("resourceID", newResource.ID),
		zap.Time("oldUpdatedAt", oldResource.UpdatedAt),
		zap.Time("newUpdatedAt", newResource.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateResourceIndexes(ctx, newResource); err != nil {
		logger.Error("Failed to update resource indexes", zap.Error(err), zap.String("resourceID", newResource.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceChange(ctx, "updated", newResource); err != nil {
		logger.Warn("Failed to send resource update notification", zap.Error(err), zap.String("resourceID", newResource.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this resource change
	if err := s.invalidateRelatedCaches(ctx, newResource.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("resourceID", newResource.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *ResourceService) handleResourceDeleted(ctx context.Context, event util.Event) error {
	resourceID := event.Payload.(string)
	logger.Info("Resource deleted event received", zap.String("resourceID", resourceID))

	// Remove resource from any indexes or materialized views
	if err := s.removeResourceFromIndexes(ctx, resourceID); err != nil {
		logger.Error("Failed to remove resource from indexes", zap.Error(err), zap.String("resourceID", resourceID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceChange(ctx, "deleted", model.Resource{ID: resourceID}); err != nil {
		logger.Warn("Failed to send resource deletion notification", zap.Error(err), zap.String("resourceID", resourceID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupResourceRelatedData(ctx, resourceID); err != nil {
		logger.Error("Failed to clean up resource-related data", zap.Error(err), zap.String("resourceID", resourceID))
		// Continue execution despite the error
	}

	return nil
}

// CreateResource handles the creation of a new resource
func (s *ResourceService) CreateResource(ctx context.Context, resource model.Resource, creatorID string) (*model.Resource, error) {
	if err := s.validationUtil.ValidateResource(resource); err != nil {
		logger.Error("Validation for resource data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid resource: %w", err)
	}

	// Check if resource with the same ID already exists
	if resource.ID != "" {
		_, err := s.resourceDAO.GetResource(ctx, resource.ID)
		if err == nil {
			// Resource with this ID already exists
			return nil, echo_errors.ErrResourceConflict
		}
		if err != echo_errors.ErrResourceNotFound {
			// An error occurred while checking for existing resource
			return nil, echo_errors.ErrDatabaseOperation
		}
	}

	resource.CreatedAt = time.Now()
	resource.UpdatedAt = time.Now()
	resource.CreatedBy = creatorID
	resource.UpdatedBy = creatorID

	resourceID, err := s.resourceDAO.CreateResource(ctx, resource)
	if err != nil {
		logger.Error("Error creating resource", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	resource.ID = resourceID

	// Update cache
	if err := s.cacheService.SetResource(ctx, resource); err != nil {
		logger.Warn("Failed to cache resource", zap.Error(err), zap.String("resourceID", resourceID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resource.created", resource)

	logger.Info("Resource created successfully", zap.String("resourceID", resourceID), zap.String("creatorID", creatorID))
	return &resource, nil
}

// UpdateResource handles updates to an existing resource
func (s *ResourceService) UpdateResource(ctx context.Context, resource model.Resource, updaterID string) (*model.Resource, error) {
	if err := s.validationUtil.ValidateResource(resource); err != nil {
		logger.Error("Validation for resource data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid resource: %w", err)
	}

	oldResource, err := s.resourceDAO.GetResource(ctx, resource.ID)
	if err != nil {
		logger.Error("Error retrieving existing resource", zap.Error(err), zap.String("resourceID", resource.ID))
		return nil, err
	}

	resource.UpdatedAt = time.Now()
	resource.UpdatedBy = updaterID

	updatedResource, err := s.resourceDAO.UpdateResource(ctx, resource)
	if err != nil {
		logger.Error("Error updating resource", zap.Error(err), zap.String("resourceID", resource.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update resource: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetResource(ctx, *updatedResource); err != nil {
		logger.Warn("Failed to update resource in cache", zap.Error(err), zap.String("resourceID", resource.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resource.updated", map[string]model.Resource{
		"old": *oldResource,
		"new": *updatedResource,
	})

	logger.Info("Resource updated successfully", zap.String("resourceID", resource.ID), zap.String("updaterID", updaterID))
	return updatedResource, nil
}

// DeleteResource handles the deletion of a resource
func (s *ResourceService) DeleteResource(ctx context.Context, resourceID string, deleterID string) error {
	err := s.resourceDAO.DeleteResource(ctx, resourceID)
	if err != nil {
		logger.Error("Error deleting resource", zap.Error(err), zap.String("resourceID", resourceID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteResource(ctx, resourceID); err != nil {
		logger.Warn("Failed to delete resource from cache", zap.Error(err), zap.String("resourceID", resourceID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resource.deleted", resourceID)

	logger.Info("Resource deleted successfully", zap.String("resourceID", resourceID), zap.String("deleterID", deleterID))
	return nil
}

// GetResource retrieves a resource by its ID
func (s *ResourceService) GetResource(ctx context.Context, resourceID string) (*model.Resource, error) {
	// Try to get from cache first
	cachedResource, err := s.cacheService.GetResource(ctx, resourceID)
	if err == nil && cachedResource != nil {
		return cachedResource, nil
	}

	resource, err := s.resourceDAO.GetResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrResourceNotFound) {
			return nil, echo_errors.ErrResourceNotFound
		}
		logger.Error("Error retrieving resource", zap.Error(err), zap.String("resourceID", resourceID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetResource(ctx, *resource); err != nil {
		logger.Warn("Failed to cache resource", zap.Error(err), zap.String("resourceID", resourceID))
	}

	return resource, nil
}

// ListResources retrieves all resources, possibly with pagination
func (s *ResourceService) ListResources(ctx context.Context, limit int, offset int) ([]*model.Resource, error) {
	resources, err := s.resourceDAO.ListResources(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing resources", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	return resources, nil
}

// SearchResources searches for resources based on criteria
func (s *ResourceService) SearchResources(ctx context.Context, criteria model.ResourceSearchCriteria) ([]*model.Resource, error) {
	logger.Info("Searching resources", zap.Any("criteria", criteria))

	if criteria.Limit < 1 {
		criteria.Limit = 10 // or any other default value
	}

	if criteria.Offset < 0 {
		criteria.Offset = 0
	}

	resources, err := s.resourceDAO.SearchResources(ctx, criteria)
	if err != nil {
		logger.Error("Error searching resources",
			zap.Error(err),
			zap.Any("criteria", criteria))
		return nil, fmt.Errorf("failed to search resources: %w", err)
	}

	logger.Info("Resources search completed", zap.Int("resourceCount", len(resources)))
	return resources, nil
}

// Helper methods

func (s *ResourceService) updateResourceIndexes(ctx context.Context, resource model.Resource) error {
	// Implementation for updating indexes
	return nil
}

func (s *ResourceService) invalidateRelatedCaches(ctx context.Context, resourceID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *ResourceService) removeResourceFromIndexes(ctx context.Context, resourceID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *ResourceService) cleanupResourceRelatedData(ctx context.Context, resourceID string) error {
	// Implementation for cleaning up related data
	return nil
}
