// api/model/access.go
package model

import "time"

type Role struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	OrganizationID string            `json:"organization_id"`
	DepartmentID   string            `json:"department_id,omitempty"` // Optional, for department-specific roles
	Permissions    []string          `json:"permissions,omitempty"`   // IDs of associated permissions
	Attributes     map[string]string `json:"attributes,omitempty"`    // For ABAC-specific attributes
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Group struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	OrganizationID string            `json:"organization_id"`
	DepartmentID   string            `json:"department_id,omitempty"` // Optional, for department-specific groups
	Roles          []string          `json:"roles,omitempty"`         // IDs of associated roles
	Attributes     map[string]string `json:"attributes,omitempty"`    // For ABAC-specific attributes
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Action      string `json:"action"` // e.g., "read", "write", "delete"
}

type DynamicAttribute struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      string      `json:"type"` // e.g., "time", "location", "device", etc.
	Value     interface{} `json:"value"`
	UpdatedAt time.Time   `json:"updated_at"`
}
