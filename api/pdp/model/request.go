package model

import (
	"time"

	"github.com/dev-mohitbeniwal/echo/api/model"
)

type AccessRequest struct {
	Subject     Subject                `json:"subject"`
	Resource    Resource               `json:"resource"`
	Action      string                 `json:"action"`
	Environment map[string]interface{} `json:"environment"`
	Timestamp   time.Time              `json:"timestamp"`
}

type Subject struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // e.g., "user", "role", "group"
	Attributes map[string]interface{} `json:"attributes"`
}

type Resource struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

// PolicyContextData represents the context data needed for policy evaluation
type PolicyContextData struct {
	Request          AccessRequest
	User             model.User
	Resource         model.Resource
	RelevantPolicies []model.Policy
}
