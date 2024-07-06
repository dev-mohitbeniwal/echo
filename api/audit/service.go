// api/audit/service.go
package audit

import (
	"context"
	"time"
)

type Service interface {
	LogAccess(ctx context.Context, log AuditLog) error
	QueryLogs(ctx context.Context, from, to time.Time, userID, resourceID string) ([]AuditLog, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) LogAccess(ctx context.Context, log AuditLog) error {
	return s.repo.LogAccess(ctx, log)
}

func (s *service) QueryLogs(ctx context.Context, from, to time.Time, userID, resourceID string) ([]AuditLog, error) {
	return s.repo.QueryLogs(ctx, from, to, userID, resourceID)
}
