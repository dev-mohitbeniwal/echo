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
