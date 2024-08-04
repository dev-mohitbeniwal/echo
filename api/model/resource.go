// api/model/resource.go
package model

import (
	"time"
)

type Resource struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Type             string            `json:"type"`          // e.g., "DOCUMENT", "APPLICATION", "API"
	TypeID           string            `json:"type_id"`       // ID of the ResourceType this resource belongs to
	URI              string            `json:"uri,omitempty"` // Uniform Resource Identifier
	OrganizationID   string            `json:"organization_id"`
	DepartmentID     string            `json:"department_id,omitempty"`
	OwnerID          string            `json:"owner_id"` // User ID of the resource owner
	Status           string            `json:"status"`   // e.g., "active", "archived", "deleted"
	Version          int               `json:"version"`
	Tags             []string          `json:"tags,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	AttributeGroupID string            `json:"attribute_group_id"` // ID of the AttributeGroup this resource belongs to
	ResourceGroupID  string            `json:"resource_group_id,omitempty"`

	// ABAC-specific attributes
	Sensitivity    string `json:"sensitivity"` // e.g., "public", "internal", "confidential", "restricted"
	Classification string `json:"classification"`
	Location       string `json:"location,omitempty"` // Physical or logical location
	Format         string `json:"format,omitempty"`   // e.g., "pdf", "docx", "json"
	Size           int64  `json:"size,omitempty"`     // Size in bytes

	// Time-based attributes
	CreatedAt      time.Time  `json:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`

	// Audit and lineage
	CreatedBy string `json:"created_by,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`

	// Access control
	ACL          []ACLEntry `json:"acl,omitempty"`           // Access Control List
	InheritedACL bool       `json:"inherited_acl,omitempty"` // Whether ACL is inherited from parent

	// Relationships
	ParentID    string   `json:"parent_id,omitempty"` // For hierarchical resources
	ChildrenIDs []string `json:"children_ids,omitempty"`
	RelatedIDs  []string `json:"related_ids,omitempty"`

	// Custom attributes for flexible ABAC policies
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type ACLEntry struct {
	SubjectID   string   `json:"subject_id"`   // User or Group ID
	SubjectType string   `json:"subject_type"` // "user" or "group"
	Permissions []string `json:"permissions"`  // List of permissions
}

type ResourceType struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by,omitempty"`
	UpdatedBy   string    `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type AttributeGroup struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
	CreatedBy  string            `json:"created_by,omitempty"`
	UpdatedBy  string            `json:"updated_by,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at,omitempty"`
}

type ResourceSearchCriteria struct {
	ID             string                 `json:"id,omitempty"`
	Name           string                 `json:"name,omitempty"`
	Type           string                 `json:"type,omitempty"`
	OrganizationID string                 `json:"organization_id,omitempty"`
	DepartmentID   string                 `json:"department_id,omitempty"`
	OwnerID        string                 `json:"owner_id,omitempty"`
	Status         string                 `json:"status,omitempty"`
	Sensitivity    string                 `json:"sensitivity,omitempty"`
	Classification string                 `json:"classification,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	CreatedAfter   *time.Time             `json:"created_after,omitempty"`
	CreatedBefore  *time.Time             `json:"created_before,omitempty"`
	UpdatedAfter   *time.Time             `json:"updated_after,omitempty"`
	UpdatedBefore  *time.Time             `json:"updated_before,omitempty"`
	Attributes     map[string]interface{} `json:"attributes,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	Offset         int                    `json:"offset,omitempty"`
	SortBy         string                 `json:"sort_by,omitempty"`
	SortOrder      string                 `json:"sort_order,omitempty"`
}
