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
