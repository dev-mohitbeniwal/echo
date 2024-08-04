// api/util/cache_service.go

package util

import (
	"context"

	"github.com/dev-mohitbeniwal/echo/api/db"
	"github.com/dev-mohitbeniwal/echo/api/model"
)

type CacheService struct{}

func NewCacheService() *CacheService {
	return &CacheService{}
}

func (c *CacheService) GetPolicy(ctx context.Context, policyID string) (*model.Policy, error) {
	return db.GetCachedPolicy(ctx, policyID)
}

func (c *CacheService) SetPolicy(ctx context.Context, policy model.Policy) error {
	return db.CachePolicy(ctx, &policy)
}

func (c *CacheService) DeletePolicy(ctx context.Context, policyID string) error {
	return db.DeleteCachedPolicy(ctx, policyID)
}

func (c *CacheService) SetOrganization(ctx context.Context, organization model.Organization) error {
	return db.CacheOrganization(ctx, &organization)
}

func (c *CacheService) DeleteOrganization(ctx context.Context, organizationID string) error {
	return db.DeleteCachedOrganization(ctx, organizationID)
}

func (c *CacheService) GetOrganization(ctx context.Context, organizationID string) (*model.Organization, error) {
	return db.GetCachedOrganization(ctx, organizationID)
}

func (c *CacheService) SetDepartment(ctx context.Context, department model.Department) error {
	return db.CacheDepartment(ctx, &department)
}

func (c *CacheService) DeleteDepartment(ctx context.Context, departmentID string) error {
	return db.DeleteCachedDepartment(ctx, departmentID)
}

func (c *CacheService) GetDepartment(ctx context.Context, departmentID string) (*model.Department, error) {
	return db.GetCachedDepartment(ctx, departmentID)
}

func (c *CacheService) SetUser(ctx context.Context, user model.User) error {
	return db.CacheUser(ctx, &user)
}

func (c *CacheService) DeleteUser(ctx context.Context, userID string) error {
	return db.DeleteCachedUser(ctx, userID)
}

func (c *CacheService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return db.GetCachedUser(ctx, userID)
}

func (c *CacheService) SetRole(ctx context.Context, role model.Role) error {
	return db.CacheRole(ctx, &role)
}

func (c *CacheService) DeleteRole(ctx context.Context, roleID string) error {
	return db.DeleteCachedRole(ctx, roleID)
}

func (c *CacheService) GetRole(ctx context.Context, roleID string) (*model.Role, error) {
	return db.GetCachedRole(ctx, roleID)
}

// SetGroup
func (c *CacheService) SetGroup(ctx context.Context, group model.Group) error {
	return db.CacheGroup(ctx, &group)
}

// DeleteGroup
func (c *CacheService) DeleteGroup(ctx context.Context, groupID string) error {
	return db.DeleteCachedGroup(ctx, groupID)
}

// GetGroup
func (c *CacheService) GetGroup(ctx context.Context, groupID string) (*model.Group, error) {
	return db.GetCachedGroup(ctx, groupID)
}

// SetPermission
func (c *CacheService) SetPermission(ctx context.Context, permission model.Permission) error {
	return db.CachePermission(ctx, &permission)
}

// DeletePermission
func (c *CacheService) DeletePermission(ctx context.Context, permissionID string) error {
	return db.DeleteCachedPermission(ctx, permissionID)
}

// GetPermission
func (c *CacheService) GetPermission(ctx context.Context, permissionID string) (*model.Permission, error) {
	return db.GetCachedPermission(ctx, permissionID)
}

// SetResource
func (c *CacheService) SetResource(ctx context.Context, resource model.Resource) error {
	return db.CacheResource(ctx, &resource)
}

// DeleteResource
func (c *CacheService) DeleteResource(ctx context.Context, resourceID string) error {
	return db.DeleteCachedResource(ctx, resourceID)
}

// GetResource
func (c *CacheService) GetResource(ctx context.Context, resourceID string) (*model.Resource, error) {
	return db.GetCachedResource(ctx, resourceID)
}

// GetResourceType
func (c *CacheService) GetResourceType(ctx context.Context, resourceTypeID string) (*model.ResourceType, error) {
	return db.GetCachedResourceType(ctx, resourceTypeID)
}

// SetResourceType
func (c *CacheService) SetResourceType(ctx context.Context, resourceType model.ResourceType) error {
	return db.CacheResourceType(ctx, &resourceType)
}

// DeleteResourceType
func (c *CacheService) DeleteResourceType(ctx context.Context, resourceTypeID string) error {
	return db.DeleteCachedResourceType(ctx, resourceTypeID)
}

// SetAttributeGroup
func (c *CacheService) SetAttributeGroup(ctx context.Context, attributeGroup model.AttributeGroup) error {
	return db.CacheAttributeGroup(ctx, &attributeGroup)
}

// DeleteAttributeGroup
func (c *CacheService) DeleteAttributeGroup(ctx context.Context, attributeGroupID string) error {
	return db.DeleteCachedAttributeGroup(ctx, attributeGroupID)
}

// GetAttributeGroup
func (c *CacheService) GetAttributeGroup(ctx context.Context, attributeGroupID string) (*model.AttributeGroup, error) {
	return db.GetCachedAttributeGroup(ctx, attributeGroupID)
}
