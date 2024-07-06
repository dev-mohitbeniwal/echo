// api/audit/model.go
package audit

import (
	"encoding/json"
	"time"
)

type AuditLog struct {
	Timestamp     time.Time       `json:"timestamp"`
	UserID        string          `json:"user_id"`
	Action        string          `json:"action"`
	ResourceID    string          `json:"resource_id"`
	AccessGranted bool            `json:"access_granted"`
	PolicyID      string          `json:"policy_id"`
	ChangeDetails json.RawMessage `json:"change_details,omitempty"`
}
