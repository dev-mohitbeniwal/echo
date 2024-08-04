// api/service/resource_type_service.go
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

// IResourceTypeService defines the interface for resource type operations
type IResourceTypeService interface {
	CreateResourceType(ctx context.Context, resourceType model.ResourceType, creatorID string) (*model.ResourceType, error)
	UpdateResourceType(ctx context.Context, resourceType model.ResourceType, updaterID string) (*model.ResourceType, error)
	DeleteResourceType(ctx context.Context, resourceTypeID string, deleterID string) error
	GetResourceType(ctx context.Context, resourceTypeID string) (*model.ResourceType, error)
	ListResourceTypes(ctx context.Context, limit int, offset int) ([]*model.ResourceType, error)
}

// ResourceTypeService handles business logic for resource type operations
type ResourceTypeService struct {
	resourceTypeDAO *dao.ResourceTypeDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IResourceTypeService = &ResourceTypeService{}

// NewResourceTypeService creates a new instance of ResourceTypeService
func NewResourceTypeService(resourceTypeDAO *dao.ResourceTypeDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *ResourceTypeService {
	service := &ResourceTypeService{
		resourceTypeDAO: resourceTypeDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("resourceType.created", service.handleResourceTypeCreated)
	eventBus.Subscribe("resourceType.updated", service.handleResourceTypeUpdated)
	eventBus.Subscribe("resourceType.deleted", service.handleResourceTypeDeleted)

	return service
}

func (s *ResourceTypeService) handleResourceTypeCreated(ctx context.Context, event util.Event) error {
	resourceType := event.Payload.(model.ResourceType)
	logger.Info("Resource type created event received", zap.String("resourceTypeID", resourceType.ID))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceTypeChange(ctx, "created", resourceType); err != nil {
		logger.Warn("Failed to send resource type creation notification", zap.Error(err), zap.String("resourceTypeID", resourceType.ID))
	}

	return nil
}

func (s *ResourceTypeService) handleResourceTypeUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.ResourceType)
	_, newResourceType := payload["old"], payload["new"]

	logger.Info("Resource type updated event received",
		zap.String("resourceTypeID", newResourceType.ID))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceTypeChange(ctx, "updated", newResourceType); err != nil {
		logger.Warn("Failed to send resource type update notification", zap.Error(err), zap.String("resourceTypeID", newResourceType.ID))
	}

	// Invalidate any caches that might be affected by this resource type change
	if err := s.invalidateRelatedCaches(ctx, newResourceType.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("resourceTypeID", newResourceType.ID))
	}

	return nil
}

func (s *ResourceTypeService) handleResourceTypeDeleted(ctx context.Context, event util.Event) error {
	resourceTypeID := event.Payload.(string)
	logger.Info("Resource type deleted event received", zap.String("resourceTypeID", resourceTypeID))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyResourceTypeChange(ctx, "deleted", model.ResourceType{ID: resourceTypeID}); err != nil {
		logger.Warn("Failed to send resource type deletion notification", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
	}

	// Clean up any related data or resources
	if err := s.cleanupResourceTypeRelatedData(ctx, resourceTypeID); err != nil {
		logger.Error("Failed to clean up resource type-related data", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
	}

	return nil
}

// CreateResourceType handles the creation of a new resource type
func (s *ResourceTypeService) CreateResourceType(ctx context.Context, resourceType model.ResourceType, creatorID string) (*model.ResourceType, error) {
	if err := s.validationUtil.ValidateResourceType(resourceType); err != nil {
		logger.Error("Validation for resource type data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid resource type: %w", err)
	}

	resourceType.CreatedAt = time.Now()
	resourceType.UpdatedAt = time.Now()
	resourceType.CreatedBy = creatorID
	resourceType.UpdatedBy = creatorID

	resourceTypeID, err := s.resourceTypeDAO.CreateResourceType(ctx, resourceType)
	if err != nil {
		logger.Error("Error creating resource type", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	resourceType.ID = resourceTypeID

	// Update cache
	if err := s.cacheService.SetResourceType(ctx, resourceType); err != nil {
		logger.Warn("Failed to cache resource type", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resourceType.created", resourceType)

	logger.Info("Resource type created successfully", zap.String("resourceTypeID", resourceTypeID), zap.String("creatorID", creatorID))
	return &resourceType, nil
}

// UpdateResourceType handles updates to an existing resource type
func (s *ResourceTypeService) UpdateResourceType(ctx context.Context, resourceType model.ResourceType, updaterID string) (*model.ResourceType, error) {
	if err := s.validationUtil.ValidateResourceType(resourceType); err != nil {
		logger.Error("Validation for resource type data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid resource type: %w", err)
	}

	oldResourceType, err := s.resourceTypeDAO.GetResourceType(ctx, resourceType.ID)
	if err != nil {
		logger.Error("Error retrieving existing resource type", zap.Error(err), zap.String("resourceTypeID", resourceType.ID))
		return nil, err
	}

	resourceType.UpdatedAt = time.Now()
	resourceType.UpdatedBy = updaterID

	updatedResourceType, err := s.resourceTypeDAO.UpdateResourceType(ctx, resourceType)
	if err != nil {
		logger.Error("Error updating resource type", zap.Error(err), zap.String("resourceTypeID", resourceType.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update resource type: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetResourceType(ctx, *updatedResourceType); err != nil {
		logger.Warn("Failed to update resource type in cache", zap.Error(err), zap.String("resourceTypeID", resourceType.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resourceType.updated", map[string]model.ResourceType{
		"old": *oldResourceType,
		"new": *updatedResourceType,
	})

	logger.Info("Resource type updated successfully", zap.String("resourceTypeID", resourceType.ID), zap.String("updaterID", updaterID))
	return updatedResourceType, nil
}

// DeleteResourceType handles the deletion of a resource type
func (s *ResourceTypeService) DeleteResourceType(ctx context.Context, resourceTypeID string, deleterID string) error {
	err := s.resourceTypeDAO.DeleteResourceType(ctx, resourceTypeID)
	if err != nil {
		logger.Error("Error deleting resource type", zap.Error(err), zap.String("resourceTypeID", resourceTypeID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete resource type: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteResourceType(ctx, resourceTypeID); err != nil {
		logger.Warn("Failed to delete resource type from cache", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "resourceType.deleted", resourceTypeID)

	logger.Info("Resource type deleted successfully", zap.String("resourceTypeID", resourceTypeID), zap.String("deleterID", deleterID))
	return nil
}

// GetResourceType retrieves a resource type by its ID
func (s *ResourceTypeService) GetResourceType(ctx context.Context, resourceTypeID string) (*model.ResourceType, error) {
	// Try to get from cache first
	cachedResourceType, err := s.cacheService.GetResourceType(ctx, resourceTypeID)
	if err == nil && cachedResourceType != nil {
		return cachedResourceType, nil
	}

	resourceType, err := s.resourceTypeDAO.GetResourceType(ctx, resourceTypeID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrResourceTypeNotFound) {
			return nil, echo_errors.ErrResourceTypeNotFound
		}
		logger.Error("Error retrieving resource type", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetResourceType(ctx, *resourceType); err != nil {
		logger.Warn("Failed to cache resource type", zap.Error(err), zap.String("resourceTypeID", resourceTypeID))
	}

	return resourceType, nil
}

// ListResourceTypes retrieves all resource types, possibly with pagination
func (s *ResourceTypeService) ListResourceTypes(ctx context.Context, limit int, offset int) ([]*model.ResourceType, error) {
	resourceTypes, err := s.resourceTypeDAO.ListResourceTypes(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing resource types", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list resource types: %w", err)
	}

	return resourceTypes, nil
}

// Helper methods

func (s *ResourceTypeService) invalidateRelatedCaches(ctx context.Context, resourceTypeID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *ResourceTypeService) cleanupResourceTypeRelatedData(ctx context.Context, resourceTypeID string) error {
	// Implementation for cleaning up related data
	return nil
}
