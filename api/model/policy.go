// api/model/policy.go
package model

import (
	"time"
)

type Policy struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Effect            string      `json:"effect"` // "allow" or "deny"
	Subjects          []Subject   `json:"subjects"`
	Resources         []Resource  `json:"resources"`
	Actions           []string    `json:"actions"`
	Conditions        []Condition `json:"conditions"`
	DynamicAttributes []string    `json:"dynamic_attributes,omitempty"`
	Priority          int         `json:"priority"`
	Version           int         `json:"version"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	Active            bool        `json:"active"`
	ActivationDate    *time.Time  `json:"activation_date,omitempty"`
	DeactivationDate  *time.Time  `json:"deactivation_date,omitempty"`
}

type Subject struct {
	Type       string            `json:"type"` // e.g., "user", "role", "group"
	UserID     string            `json:"user_id,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

type Resource struct {
	Type       string            `json:"type"` // e.g., "file", "database", "api"
	Attributes map[string]string `json:"attributes"`
}

type Condition struct {
	Attribute     string        `json:"attribute"`
	Operator      string        `json:"operator"`
	Value         interface{}   `json:"value"`
	SubConditions *ConditionSet `json:"sub_conditions,omitempty"`
	IsDynamic     bool          `json:"is_dynamic"` // Add this field
}

type ConditionSet struct {
	Operator   string      `json:"operator"` // "AND" or "OR"
	Conditions []Condition `json:"conditions"`
}

// New types for Neo4j relationships

type AppliesTo struct {
	PolicyID  string `json:"policy_id"`
	SubjectID string `json:"subject_id"`
}

type Governs struct {
	PolicyID   string `json:"policy_id"`
	ResourceID string `json:"resource_id"`
}

type HasCondition struct {
	PolicyID    string `json:"policy_id"`
	ConditionID string `json:"condition_id"`
}

type BelongsTo struct {
	SubjectID      string `json:"subject_id"`
	OrganizationID string `json:"organization_id"`
}

type PartOf struct {
	ResourceID     string `json:"resource_id"`
	OrganizationID string `json:"organization_id"`
}

type PolicySearchCriteria struct {
	Name        string
	Effect      string
	MinPriority int
	MaxPriority int
	Active      *bool
	FromDate    time.Time
	ToDate      time.Time
	Limit       int
}

type PolicyUsageAnalysis struct {
	PolicyID       string
	PolicyName     string
	ResourceCount  int
	SubjectCount   int
	ConditionCount int
	CreatedAt      time.Time
	LastUpdatedAt  time.Time
}
