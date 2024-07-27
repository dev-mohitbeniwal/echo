// api/service/group_service.go
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

// IGroupService defines the interface for group operations
type IGroupService interface {
	CreateGroup(ctx context.Context, group model.Group, creatorID string) (*model.Group, error)
	UpdateGroup(ctx context.Context, group model.Group, updaterID string) (*model.Group, error)
	DeleteGroup(ctx context.Context, groupID string, deleterID string) error
	GetGroup(ctx context.Context, groupID string) (*model.Group, error)
	ListGroups(ctx context.Context, limit int, offset int) ([]*model.Group, error)
	SearchGroups(ctx context.Context, query string, limit, offset int) ([]*model.Group, error)
}

// GroupService handles business logic for group operations
type GroupService struct {
	groupDAO        *dao.GroupDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IGroupService = &GroupService{}

// NewGroupService creates a new instance of GroupService
func NewGroupService(groupDAO *dao.GroupDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *GroupService {
	service := &GroupService{
		groupDAO:        groupDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("group.created", service.handleGroupCreated)
	eventBus.Subscribe("group.updated", service.handleGroupUpdated)
	eventBus.Subscribe("group.deleted", service.handleGroupDeleted)

	return service
}

func (s *GroupService) handleGroupCreated(ctx context.Context, event util.Event) error {
	group := event.Payload.(model.Group)
	logger.Info("Group created event received", zap.String("groupID", group.ID))

	// Update any indexes or materialized views
	if err := s.updateGroupIndexes(ctx, group); err != nil {
		logger.Error("Failed to update group indexes", zap.Error(err), zap.String("groupID", group.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyGroupChange(ctx, "created", group); err != nil {
		logger.Warn("Failed to send group creation notification", zap.Error(err), zap.String("groupID", group.ID))
	}

	return nil
}

func (s *GroupService) handleGroupUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Group)
	oldGroup, newGroup := payload["old"], payload["new"]

	logger.Info("Group updated event received",
		zap.String("groupID", newGroup.ID),
		zap.Time("oldUpdatedAt", oldGroup.UpdatedAt),
		zap.Time("newUpdatedAt", newGroup.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateGroupIndexes(ctx, newGroup); err != nil {
		logger.Error("Failed to update group indexes", zap.Error(err), zap.String("groupID", newGroup.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyGroupChange(ctx, "updated", newGroup); err != nil {
		logger.Warn("Failed to send group update notification", zap.Error(err), zap.String("groupID", newGroup.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this group change
	if err := s.invalidateRelatedCaches(ctx, newGroup.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("groupID", newGroup.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *GroupService) handleGroupDeleted(ctx context.Context, event util.Event) error {
	groupID := event.Payload.(string)
	logger.Info("Group deleted event received", zap.String("groupID", groupID))

	// Remove group from any indexes or materialized views
	if err := s.removeGroupFromIndexes(ctx, groupID); err != nil {
		logger.Error("Failed to remove group from indexes", zap.Error(err), zap.String("groupID", groupID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyGroupChange(ctx, "deleted", model.Group{ID: groupID}); err != nil {
		logger.Warn("Failed to send group deletion notification", zap.Error(err), zap.String("groupID", groupID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupGroupRelatedData(ctx, groupID); err != nil {
		logger.Error("Failed to clean up group-related data", zap.Error(err), zap.String("groupID", groupID))
		// Continue execution despite the error
	}

	return nil
}

// CreateGroup handles the creation of a new group
func (s *GroupService) CreateGroup(ctx context.Context, group model.Group, creatorID string) (*model.Group, error) {
	if err := s.validationUtil.ValidateGroup(group); err != nil {
		return nil, fmt.Errorf("invalid group: %w", err)
	}

	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	groupID, err := s.groupDAO.CreateGroup(ctx, group)
	if err != nil {
		logger.Error("Error creating group", zap.Error(err), zap.String("creatorID", creatorID))
		return nil, err
	}

	group.ID = groupID

	// Update cache
	if err := s.cacheService.SetGroup(ctx, group); err != nil {
		logger.Warn("Failed to cache group", zap.Error(err), zap.String("groupID", groupID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "group.created", group)

	logger.Info("Group created successfully", zap.String("groupID", groupID), zap.String("creatorID", creatorID))
	return &group, nil
}

// UpdateGroup handles updates to an existing group
func (s *GroupService) UpdateGroup(ctx context.Context, group model.Group, updaterID string) (*model.Group, error) {
	if err := s.validationUtil.ValidateGroup(group); err != nil {
		return nil, fmt.Errorf("invalid group: %w", err)
	}

	oldGroup, err := s.groupDAO.GetGroup(ctx, group.ID)
	if err != nil {
		logger.Error("Error retrieving existing group", zap.Error(err), zap.String("groupID", group.ID))
		return nil, err
	}

	group.UpdatedAt = time.Now()

	updatedGroup, err := s.groupDAO.UpdateGroup(ctx, group)
	if err != nil {
		logger.Error("Error updating group", zap.Error(err), zap.String("groupID", group.ID), zap.String("updaterID", updaterID))
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetGroup(ctx, *updatedGroup); err != nil {
		logger.Warn("Failed to update group in cache", zap.Error(err), zap.String("groupID", group.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "group.updated", map[string]model.Group{
		"old": *oldGroup,
		"new": *updatedGroup,
	})

	logger.Info("Group updated successfully", zap.String("groupID", group.ID), zap.String("updaterID", updaterID))
	return updatedGroup, nil
}

// DeleteGroup handles the deletion of a group
func (s *GroupService) DeleteGroup(ctx context.Context, groupID string, deleterID string) error {
	err := s.groupDAO.DeleteGroup(ctx, groupID)
	if err != nil {
		logger.Error("Error deleting group", zap.Error(err), zap.String("groupID", groupID), zap.String("deleterID", deleterID))
		return fmt.Errorf("failed to delete group: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteGroup(ctx, groupID); err != nil {
		logger.Warn("Failed to delete group from cache", zap.Error(err), zap.String("groupID", groupID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "group.deleted", groupID)

	logger.Info("Group deleted successfully", zap.String("groupID", groupID), zap.String("deleterID", deleterID))
	return nil
}

// GetGroup retrieves a group by its ID
func (s *GroupService) GetGroup(ctx context.Context, groupID string) (*model.Group, error) {
	// Try to get from cache first
	cachedGroup, err := s.cacheService.GetGroup(ctx, groupID)
	if err == nil && cachedGroup != nil {
		return cachedGroup, nil
	}

	group, err := s.groupDAO.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrGroupNotFound) {
			return nil, echo_errors.ErrGroupNotFound
		}
		logger.Error("Error retrieving group", zap.Error(err), zap.String("groupID", groupID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetGroup(ctx, *group); err != nil {
		logger.Warn("Failed to cache group", zap.Error(err), zap.String("groupID", groupID))
	}

	return group, nil
}

// ListGroups retrieves all groups, possibly with pagination
func (s *GroupService) ListGroups(ctx context.Context, limit int, offset int) ([]*model.Group, error) {
	groups, err := s.groupDAO.ListGroups(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing groups", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	return groups, nil
}

// SearchGroups searches for groups based on a query string
func (s *GroupService) SearchGroups(ctx context.Context, query string, limit, offset int) ([]*model.Group, error) {
	// Implement group search logic here
	// This might involve searching by group name, description, or other attributes
	// You may need to add a corresponding method in the GroupDAO
	return nil, fmt.Errorf("group search not implemented")
}

// Helper methods

func (s *GroupService) updateGroupIndexes(ctx context.Context, group model.Group) error {
	// Implementation for updating indexes
	return nil
}

func (s *GroupService) invalidateRelatedCaches(ctx context.Context, groupID string) error {
	// Implementation for invalidating caches
	return nil
}

func (s *GroupService) removeGroupFromIndexes(ctx context.Context, groupID string) error {
	// Implementation for removing from indexes
	return nil
}

func (s *GroupService) cleanupGroupRelatedData(ctx context.Context, groupID string) error {
	// Implementation for cleaning up related data
	return nil
}
