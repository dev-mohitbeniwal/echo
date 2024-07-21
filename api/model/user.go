// api/model/user.go
package model

import "time"

type User struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Username       string            `json:"username"`
	Email          string            `json:"email"`
	Password       string            `json:"-"`         // Hashed password, not returned in JSON
	UserType       string            `json:"user_type"` // "AliveLife", "CorporateAdmin", "DepartmentUser"
	OrganizationID string            `json:"organization_id,omitempty"`
	DepartmentID   string            `json:"department_id,omitempty"`
	RoleIds        []string          `json:"roles,omitempty"`       // List of role IDs
	GroupIds       []string          `json:"groups,omitempty"`      // List of group IDs
	Permissions    []string          `json:"permissions,omitempty"` // List of permission IDs (Relationship to resources)
	Attributes     map[string]string `json:"attributes"`
	Status         string            `json:"status"` // "Active", "Inactive", "Suspended", etc.
	LastLogin      *time.Time        `json:"last_login,omitempty"`
	FailedLogins   int               `json:"failed_logins"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CreatedBy      string            `json:"created_by,omitempty"` // ID of the user who created this user
	UpdatedBy      string            `json:"updated_by,omitempty"` // ID of the user who last updated this user
	DeletedAt      *time.Time        `json:"deleted_at,omitempty"` // For soft delete
}

// UserRelationships represents the relationships a user has in the graph database
type UserRelationships struct {
	WorksFor  *Organization `json:"works_for,omitempty"`
	MemberOf  *Department   `json:"member_of,omitempty"`
	HasRoles  []*Role       `json:"has_roles,omitempty"`
	BelongsTo []*Group      `json:"belongs_to,omitempty"`
	CreatedBy *User         `json:"created_by,omitempty"`
	UpdatedBy *User         `json:"updated_by,omitempty"`
}

// FullUser combines User data with its relationships
type FullUser struct {
	*User
	Relationships UserRelationships `json:"relationships,omitempty"`
}

// UserSearchCriteria defines the possible search parameters for users
type UserSearchCriteria struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Username       string            `json:"username,omitempty"`
	Email          string            `json:"email,omitempty"`
	UserType       string            `json:"user_type,omitempty"`
	OrganizationID string            `json:"organization_id,omitempty"`
	DepartmentID   string            `json:"department_id,omitempty"`
	RoleID         string            `json:"role_id,omitempty"`
	GroupID        string            `json:"group_id,omitempty"`
	Status         string            `json:"status,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	FromDate       *time.Time        `json:"from_date,omitempty"`
	ToDate         *time.Time        `json:"to_date,omitempty"`
	LastLoginAfter *time.Time        `json:"last_login_after,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	Offset         int               `json:"offset,omitempty"`
	SortBy         string            `json:"sort_by,omitempty"`
	SortOrder      string            `json:"sort_order,omitempty"`
}

type BelongsToDepartment struct {
	UserID       string `json:"user_id"`
	DepartmentID string `json:"department_id"`
}
