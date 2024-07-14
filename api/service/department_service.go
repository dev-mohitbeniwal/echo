// api/service/department_service.go
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

// IDepartmentService defines the interface for department operations
type IDepartmentService interface {
	CreateDepartment(ctx context.Context, dept model.Department, userID string) (*model.Department, error)
	UpdateDepartment(ctx context.Context, dept model.Department, userID string) (*model.Department, error)
	DeleteDepartment(ctx context.Context, deptID string, userID string) error
	GetDepartment(ctx context.Context, deptID string) (*model.Department, error)
	ListDepartments(ctx context.Context, limit int, offset int) ([]*model.Department, error)
	GetDepartmentsByOrganization(ctx context.Context, orgID string) ([]*model.Department, error)
	GetDepartmentHierarchy(ctx context.Context, deptID string) ([]*model.Department, error)
	GetChildDepartments(ctx context.Context, parentDeptID string) ([]*model.Department, error)
	MoveDepartment(ctx context.Context, deptID string, newParentID string, userID string) error
	SearchDepartments(ctx context.Context, namePattern string, limit int) ([]*model.Department, error)
}

// DepartmentService handles business logic for department operations
type DepartmentService struct {
	deptDAO         *dao.DepartmentDAO
	validationUtil  *util.ValidationUtil
	cacheService    *util.CacheService
	notificationSvc *util.NotificationService
	eventBus        *util.EventBus
}

var _ IDepartmentService = &DepartmentService{}

// NewDepartmentService creates a new instance of DepartmentService
func NewDepartmentService(deptDAO *dao.DepartmentDAO, validationUtil *util.ValidationUtil, cacheService *util.CacheService, notificationSvc *util.NotificationService, eventBus *util.EventBus) *DepartmentService {
	service := &DepartmentService{
		deptDAO:         deptDAO,
		validationUtil:  validationUtil,
		cacheService:    cacheService,
		notificationSvc: notificationSvc,
		eventBus:        eventBus,
	}

	// Set up event subscriptions
	eventBus.Subscribe("department.created", service.handleDepartmentCreated)
	eventBus.Subscribe("department.updated", service.handleDepartmentUpdated)
	eventBus.Subscribe("department.deleted", service.handleDepartmentDeleted)

	return service
}

func (s *DepartmentService) handleDepartmentCreated(ctx context.Context, event util.Event) error {
	dept := event.Payload.(model.Department)
	logger.Info("Department created event received", zap.String("deptID", dept.ID))

	// Update any indexes or materialized views
	if err := s.updateDepartmentIndexes(ctx, dept); err != nil {
		logger.Error("Failed to update department indexes", zap.Error(err), zap.String("deptID", dept.ID))
		return err
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyDepartmentChange(ctx, "created", dept); err != nil {
		logger.Warn("Failed to send department creation notification", zap.Error(err), zap.String("deptID", dept.ID))
	}

	return nil
}

func (s *DepartmentService) handleDepartmentUpdated(ctx context.Context, event util.Event) error {
	payload := event.Payload.(map[string]model.Department)
	oldDept, newDept := payload["old"], payload["new"]

	logger.Info("Department updated event received",
		zap.String("deptID", newDept.ID),
		zap.Time("oldUpdatedAt", oldDept.UpdatedAt),
		zap.Time("newUpdatedAt", newDept.UpdatedAt))

	// Update any indexes or materialized views
	if err := s.updateDepartmentIndexes(ctx, newDept); err != nil {
		logger.Error("Failed to update department indexes", zap.Error(err), zap.String("deptID", newDept.ID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyDepartmentChange(ctx, "updated", newDept); err != nil {
		logger.Warn("Failed to send department update notification", zap.Error(err), zap.String("deptID", newDept.ID))
		// Continue execution despite the error
	}

	// Invalidate any caches that might be affected by this department change
	if err := s.invalidateRelatedCaches(ctx, newDept.ID); err != nil {
		logger.Error("Failed to invalidate related caches", zap.Error(err), zap.String("deptID", newDept.ID))
		// Continue execution despite the error
	}

	return nil
}

func (s *DepartmentService) handleDepartmentDeleted(ctx context.Context, event util.Event) error {
	deptID := event.Payload.(string)
	logger.Info("Department deleted event received", zap.String("deptID", deptID))

	// Remove department from any indexes or materialized views
	if err := s.removeDepartmentFromIndexes(ctx, deptID); err != nil {
		logger.Error("Failed to remove department from indexes", zap.Error(err), zap.String("deptID", deptID))
		// Continue execution despite the error
	}

	// Notify relevant services or systems
	if err := s.notificationSvc.NotifyDepartmentChange(ctx, "deleted", model.Department{ID: deptID}); err != nil {
		logger.Warn("Failed to send department deletion notification", zap.Error(err), zap.String("deptID", deptID))
		// Continue execution despite the error
	}

	// Clean up any related data or resources
	if err := s.cleanupDepartmentRelatedData(ctx, deptID); err != nil {
		logger.Error("Failed to clean up department-related data", zap.Error(err), zap.String("deptID", deptID))
		// Continue execution despite the error
	}

	return nil
}

// CreateDepartment handles the creation of a new department
func (s *DepartmentService) CreateDepartment(ctx context.Context, dept model.Department, userID string) (*model.Department, error) {
	if err := s.validationUtil.ValidateDepartment(dept); err != nil {
		return nil, fmt.Errorf("invalid department: %w", err)
	}

	dept.CreatedAt = time.Now()
	dept.UpdatedAt = time.Now()

	deptID, err := s.deptDAO.CreateDepartment(ctx, dept)
	if err != nil {
		logger.Error("Error creating department", zap.Error(err), zap.String("userID", userID))
		return nil, err
	}

	dept.ID = deptID

	// Update cache
	if err := s.cacheService.SetDepartment(ctx, dept); err != nil {
		logger.Warn("Failed to cache department", zap.Error(err), zap.String("deptID", deptID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "department.created", dept)

	logger.Info("Department created successfully", zap.String("deptID", deptID), zap.String("userID", userID))
	return &dept, nil
}

// UpdateDepartment handles updates to an existing department
func (s *DepartmentService) UpdateDepartment(ctx context.Context, dept model.Department, userID string) (*model.Department, error) {
	if err := s.validationUtil.ValidateDepartment(dept); err != nil {
		return nil, fmt.Errorf("invalid department: %w", err)
	}

	oldDept, err := s.deptDAO.GetDepartment(ctx, dept.ID)
	if err != nil {
		logger.Error("Error retrieving existing department", zap.Error(err), zap.String("deptID", dept.ID))
		return nil, err
	}

	dept.UpdatedAt = time.Now()

	updatedDept, err := s.deptDAO.UpdateDepartment(ctx, dept)
	if err != nil {
		logger.Error("Error updating department", zap.Error(err), zap.String("deptID", dept.ID), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to update department: %w", err)
	}

	// Update cache
	if err := s.cacheService.SetDepartment(ctx, *updatedDept); err != nil {
		logger.Warn("Failed to update department in cache", zap.Error(err), zap.String("deptID", dept.ID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "department.updated", map[string]model.Department{
		"old": *oldDept,
		"new": *updatedDept,
	})

	logger.Info("Department updated successfully", zap.String("deptID", dept.ID), zap.String("userID", userID))
	return updatedDept, nil
}

// DeleteDepartment handles the deletion of a department
func (s *DepartmentService) DeleteDepartment(ctx context.Context, deptID string, userID string) error {
	err := s.deptDAO.DeleteDepartment(ctx, deptID)
	if err != nil {
		logger.Error("Error deleting department", zap.Error(err), zap.String("deptID", deptID), zap.String("userID", userID))
		return fmt.Errorf("failed to delete department: %w", err)
	}

	// Remove from cache
	if err := s.cacheService.DeleteDepartment(ctx, deptID); err != nil {
		logger.Warn("Failed to delete department from cache", zap.Error(err), zap.String("deptID", deptID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "department.deleted", deptID)

	logger.Info("Department deleted successfully", zap.String("deptID", deptID), zap.String("userID", userID))
	return nil
}

// GetDepartment retrieves a department by its ID
func (s *DepartmentService) GetDepartment(ctx context.Context, deptID string) (*model.Department, error) {
	// Try to get from cache first
	cachedDept, err := s.cacheService.GetDepartment(ctx, deptID)
	if err == nil && cachedDept != nil {
		return cachedDept, nil
	}

	dept, err := s.deptDAO.GetDepartment(ctx, deptID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrDepartmentNotFound) {
			return nil, echo_errors.ErrDepartmentNotFound
		}
		logger.Error("Error retrieving department", zap.Error(err), zap.String("deptID", deptID))
		return nil, echo_errors.ErrInternalServer
	}

	// Update cache
	if err := s.cacheService.SetDepartment(ctx, *dept); err != nil {
		logger.Warn("Failed to cache department", zap.Error(err), zap.String("deptID", deptID))
	}

	return dept, nil
}

// ListDepartments retrieves all departments, possibly with pagination
func (s *DepartmentService) ListDepartments(ctx context.Context, limit int, offset int) ([]*model.Department, error) {
	depts, err := s.deptDAO.ListDepartments(ctx, limit, offset)
	if err != nil {
		logger.Error("Error listing departments", zap.Error(err), zap.Int("limit", limit), zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}

	return depts, nil
}

// GetDepartmentsByOrganization retrieves all departments for a given organization
func (s *DepartmentService) GetDepartmentsByOrganization(ctx context.Context, orgID string) ([]*model.Department, error) {
	depts, err := s.deptDAO.GetDepartmentsByOrganization(ctx, orgID)
	if err != nil {
		logger.Error("Error retrieving departments by organization", zap.Error(err), zap.String("orgID", orgID))
		return nil, fmt.Errorf("failed to retrieve departments by organization: %w", err)
	}

	return depts, nil
}

// GetDepartmentHierarchy retrieves the department hierarchy for a given department
func (s *DepartmentService) GetDepartmentHierarchy(ctx context.Context, deptID string) ([]*model.Department, error) {
	hierarchy, err := s.deptDAO.GetDepartmentHierarchy(ctx, deptID)
	if err != nil {
		logger.Error("Error retrieving department hierarchy", zap.Error(err), zap.String("deptID", deptID))
		return nil, fmt.Errorf("failed to retrieve department hierarchy: %w", err)
	}

	return hierarchy, nil
}

// GetChildDepartments retrieves all immediate child departments of a given department
func (s *DepartmentService) GetChildDepartments(ctx context.Context, parentDeptID string) ([]*model.Department, error) {
	childDepts, err := s.deptDAO.GetChildDepartments(ctx, parentDeptID)
	if err != nil {
		logger.Error("Error retrieving child departments", zap.Error(err), zap.String("parentDeptID", parentDeptID))
		return nil, fmt.Errorf("failed to retrieve child departments: %w", err)
	}

	return childDepts, nil
}

// MoveDepartment moves a department to a new parent department
func (s *DepartmentService) MoveDepartment(ctx context.Context, deptID string, newParentID string, userID string) error {
	err := s.deptDAO.MoveDepartment(ctx, deptID, newParentID)
	if err != nil {
		logger.Error("Error moving department", zap.Error(err), zap.String("deptID", deptID), zap.String("newParentID", newParentID), zap.String("userID", userID))
		return fmt.Errorf("failed to move department: %w", err)
	}

	// Invalidate related caches
	if err := s.invalidateRelatedCaches(ctx, deptID); err != nil {
		logger.Warn("Failed to invalidate related caches after moving department", zap.Error(err), zap.String("deptID", deptID))
	}

	// Publish event for asynchronous processing
	s.eventBus.Publish(ctx, "department.moved", map[string]string{"deptID": deptID, "newParentID": newParentID})

	logger.Info("Department moved successfully", zap.String("deptID", deptID), zap.String("newParentID", newParentID), zap.String("userID", userID))
	return nil
}

// SearchDepartments searches for departments based on a name pattern
func (s *DepartmentService) SearchDepartments(ctx context.Context, namePattern string, limit int) ([]*model.Department, error) {
	depts, err := s.deptDAO.SearchDepartments(ctx, namePattern, limit)
	if err != nil {
		logger.Error("Error searching departments", zap.Error(err), zap.String("namePattern", namePattern), zap.Int("limit", limit))
		return nil, fmt.Errorf("failed to search departments: %w", err)
	}

	return depts, nil
}

// Helper methods

func (s *DepartmentService) updateDepartmentIndexes(ctx context.Context, dept model.Department) error {
	// Implementation for updating indexes
	// This could involve updating search indexes or other data structures for efficient querying
	logger.Info("Updating department indexes", zap.String("deptID", dept.ID))
	// Add your implementation here
	return nil
}

func (s *DepartmentService) invalidateRelatedCaches(ctx context.Context, deptID string) error {
	// Implementation for invalidating caches
	logger.Info("Invalidating related caches", zap.String("deptID", deptID))
	// This could involve clearing caches for the department and its related entities
	if err := s.cacheService.DeleteDepartment(ctx, deptID); err != nil {
		logger.Warn("Failed to delete department from cache", zap.Error(err), zap.String("deptID", deptID))
	}
	// Add more cache invalidation logic as needed
	return nil
}

func (s *DepartmentService) removeDepartmentFromIndexes(ctx context.Context, deptID string) error {
	// Implementation for removing from indexes
	logger.Info("Removing department from indexes", zap.String("deptID", deptID))
	// This could involve removing the department from search indexes or other data structures
	// Add your implementation here
	return nil
}

func (s *DepartmentService) cleanupDepartmentRelatedData(ctx context.Context, deptID string) error {
	// Implementation for cleaning up related data
	logger.Info("Cleaning up department-related data", zap.String("deptID", deptID))
	// This could involve removing or updating related entities, such as employees or subdepartments
	// Add your implementation here
	return nil
}

// Additional helper methods as needed

func (s *DepartmentService) validateDepartmentHierarchy(ctx context.Context, dept model.Department) error {
	// Implementation for validating the department hierarchy
	// This could check for circular references or other invalid hierarchical structures
	logger.Info("Validating department hierarchy", zap.String("deptID", dept.ID))
	// Add your implementation here
	return nil
}

func (s *DepartmentService) updateDepartmentStats(ctx context.Context, deptID string) error {
	// Implementation for updating department statistics
	// This could involve recalculating employee counts, budget allocations, etc.
	logger.Info("Updating department statistics", zap.String("deptID", deptID))
	// Add your implementation here
	return nil
}

// You can add more methods here as needed for your specific business logic
