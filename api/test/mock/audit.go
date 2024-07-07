// test/mock/audit.go
package mock

import (
	"context"
	"time"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	"github.com/stretchr/testify/mock"
)

// MockAuditService is a mock implementation of audit.Service
type MockAuditService struct {
	mock.Mock
}

func (m *MockAuditService) LogAccess(ctx context.Context, log audit.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockAuditService) QueryLogs(ctx context.Context, from, to time.Time, userID, resourceID string) ([]audit.AuditLog, error) {
	args := m.Called(ctx, from, to, userID, resourceID)
	return args.Get(0).([]audit.AuditLog), args.Error(1)
}
