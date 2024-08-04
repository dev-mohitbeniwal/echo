// api/service/attribute_group_service.go
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

// IAttributeGroupService defines the interface for attribute group operations
type IAttributeGroupService interface {
	CreateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup, creatorID string) (*model.AttributeGroup, error)
	UpdateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup, updaterID string) (*model.AttributeGroup, error)
	DeleteAttributeGroup(ctx context.Context, attributeGroupID string, deleterID string) error
	GetAttributeGroup(ctx context.Context, attributeGroupID string) (*model.AttributeGroup, error)
	ListAttributeGroups(ctx context.Context, limit int, offset int) ([]*model.AttributeGroup, error)
}

// AttributeGroupService handles business logic for attribute group operations
type AttributeGroupService struct {
	attributeGroupDAO *dao.AttributeGroupDAO
	validationUtil    *util.ValidationUtil
	cacheService      *util.CacheService
	notificationSvc   *util.NotificationService
	eventBus          *util.EventBus
}

var _ IAttributeGroupService = &AttributeGroupService{}

// NewAttributeGroupService creates a new instance of AttributeGroupService
func NewAttributeGroupService(attributeGroupDAO *dao.AttributeGroupDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *AttributeGroupService {
	service := &AttributeGroupService{
		attributeGroupDAO: attributeGroupDAO,
		validationUtil:    validationUtil,
		cacheService:      cacheService,
		notificationSvc:   notificationSvc,
		eventBus:          eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("attributeGroup.created", service.handleAttributeGroupCreated)
	eventBus.Subscribe("attributeGroup.updated", service.handleAttributeGroupUpdated)
	eventBus.Subscribe("attributeGroup.deleted", service.handleAttributeGroupDeleted)

	return service
}

func (s *AttributeGroupService) handleAttributeGroupCreated(ctx context.Context, event util.Event) error {
	attributeGroup := event.Payload.(model.AttributeGroup)
	logger.Info("Attribute group created event received", zap.String("attributeGroupID", attributeGroup.ID))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyAttributeGroupChange(ctx, "created", attributeGroup); err != nil {
		logger.Warn("Failed to send attribute group creation notification", zap.Error(err), zap.String("attributeGroupID", attributeGroup.ID))
	}

	return nil
}

func (s *AttributeGroupService) handleAttributeGroupUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.AttributeGroup)
	oldAttributeGroup, newAttributeGroup := payload["old"], payload["new"]

	logger.Info("Attribute group updated event received",
		zap.String("attributeGroupID", newAttributeGroup.ID),
		zap.Time("oldUpdatedAt", oldAttributeGroup.UpdatedAt),
		zap.Time("newUpdatedAt", newAttributeGroup.UpdatedAt))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyAttributeGroupChange(ctx, "updated", newAttributeGroup); err != nil {
		logger.Warn("Failed to send attribute group update notification", zap.Error(err), zap.String("attributeGroupID", newAttributeGroup.ID))
	}

	// Invalidate any caches that might be affected by this attribute group change
	if err := s.invalidateRelatedCaches(ctx, newAttributeGroup.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("attributeGroupID", newAttributeGroup.ID))
	}

	return nil
}

func (s *AttributeGroupService) handleAttributeGroupDeleted(ctx context.Context, event util.Event) error {
	attributeGroupID := event.Payload.(string)
	logger.Info("Attribute group deleted event received", zap.String("attributeGroupID", attributeGroupID))

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyAttributeGroupChange(ctx, "deleted", model.AttributeGroup{ID: attributeGroupID}); err != nil {
		logger.Warn("Failed to send attribute group deletion notification", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
	}

	// Clean up any related data or resources
	if err := s.cleanupAttributeGroupRelatedData(ctx, attributeGroupID); err != nil {
		logger.Error("Failed to clean up attribute group-related data", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
	}

	return nil
}

// CreateAttributeGroup handles the creation of a new attribute group
func (s *AttributeGroupService) CreateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup, creatorID string) (*model.AttributeGroup, error) {
	if err := s.validationUtil.ValidateAttributeGroup(attributeGroup); err != nil {
		logger.Error("Validation for attribute group data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid attribute group: %w", err)
	}

	attributeGroup.CreatedAt = time.Now()
	attributeGroup.UpdatedAt = time.Now()
	attributeGroup.CreatedBy = creatorID
	attributeGroup.UpdatedBy = creatorID

	attributeGroupID, err := s.attributeGroupDAO.CreateAttributeGroup(ctx, attributeGroup)
	if err != nil {
		logger.Error("Error creating attribute group", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	attributeGroup.ID = attributeGroupID

	// Update cache
	if err := s.cacheService.SetAttributeGroup(ctx, attributeGroup); err != nil {
		logger.Warn("Failed to cache attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "attributeGroup.created", attributeGroup)

	logger.Info("Attribute group created successfully", zap.String("attributeGroupID", attributeGroupID), zap.String("creatorID", creatorID))
	return &attributeGroup, nil
}

// UpdateAttributeGroup handles updates to an existing attribute group
func (s *AttributeGroupService) UpdateAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup, updaterID string) (*model.AttributeGroup, error) {
	if err := s.validationUtil.ValidateAttributeGroup(attributeGroup); err != nil {
		logger.Error("Validation for attribute group data failed", zap.Error(err))
		return nil, fmt.Errorf("invalid attribute group: %w", err)
	}

	oldAttributeGroup, err := s.attributeGroupDAO.GetAttributeGroup(ctx, attributeGroup.ID)
	if err != nil {
		logger.Error("Error retrieving existing attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroup.ID))
		return nil, err
	}

	attributeGroup.UpdatedAt = time.Now()
	attributeGroup.UpdatedBy = updaterID

	updatedAttributeGroup, err := s.attributeGroupDAO.UpdateAttributeGroup(ctx, attributeGroup)
	if err != nil {
		logger.Error("Error updating attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroup.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update attribute group: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetAttributeGroup(ctx, *updatedAttributeGroup); err != nil {
		logger.Warn("Failed to update attribute group in cache", zap.Error(err), zap.String("attributeGroupID", attributeGroup.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "attributeGroup.updated", map[string]model.AttributeGroup{
		"old": *oldAttributeGroup,
		"new": *updatedAttributeGroup,
	})

	logger.Info("Attribute group updated successfully", zap.String("attributeGroupID", attributeGroup.ID), zap.String("updaterID", updaterID))
	return updatedAttributeGroup, nil
}

// DeleteAttributeGroup handles the deletion of an attribute group
func (s *AttributeGroupService) DeleteAttributeGroup(ctx context.Context, attributeGroupID string, deleterID string) error {
	err := s.attributeGroupDAO.DeleteAttributeGroup(ctx, attributeGroupID)
	if err != nil {
		logger.Error("Error deleting attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroupID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete attribute group: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteAttributeGroup(ctx, attributeGroupID); err != nil {
		logger.Warn("Failed to delete attribute group from cache", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "attributeGroup.deleted", attributeGroupID)

	logger.Info("Attribute group deleted successfully", zap.String("attributeGroupID", attributeGroupID), zap.String("deleterID", deleterID))
	return nil
}

// GetAttributeGroup retrieves an attribute group by its ID
func (s *AttributeGroupService) GetAttributeGroup(ctx context.Context, attributeGroupID string) (*model.AttributeGroup, error) {
	// Try to get from cache first
	cachedAttributeGroup, err := s.cacheService.GetAttributeGroup(ctx, attributeGroupID)
	if err == nil && cachedAttributeGroup != nil {
		return cachedAttributeGroup, nil
	}

	attributeGroup, err := s.attributeGroupDAO.GetAttributeGroup(ctx, attributeGroupID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrAttributeGroupNotFound) {
			return nil, echo_errors.ErrAttributeGroupNotFound
		}
		logger.Error("Error retrieving attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetAttributeGroup(ctx, *attributeGroup); err != nil {
		logger.Warn("Failed to cache attribute group", zap.Error(err), zap.String("attributeGroupID", attributeGroupID))
	}

	return attributeGroup, nil
}

// ListAttributeGroups retrieves all attribute groups, possibly with pagination
func (s *AttributeGroupService) ListAttributeGroups(ctx context.Context, limit int, offset int) ([]*model.AttributeGroup, error) {
	attributeGroups, err := s.attributeGroupDAO.ListAttributeGroups(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing attribute groups", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list attribute groups: %w", err)
	}

	return attributeGroups, nil
}

// Helper methods

func (s *AttributeGroupService) invalidateRelatedCaches(ctx context.Context, attributeGroupID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *AttributeGroupService) cleanupAttributeGroupRelatedData(ctx context.Context, attributeGroupID string) error {
	// Implementation for cleaning up related data
	return nil
}
