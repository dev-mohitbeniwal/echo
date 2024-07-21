package model

import "time"

type User struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Username       string            `json:"username"`
	Email          string            `json:"email"`
	UserType       string            `json:"user_type"` // "AliveLife", "CorporateAdmin", "DepartmentUser"
	OrganizationID string            `json:"organization_id,omitempty"`
	DepartmentID   string            `json:"department_id,omitempty"`
	Attributes     map[string]string `json:"attributes"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type UserSearchCriteria struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Username       string            `json:"username,omitempty"`
	Email          string            `json:"email,omitempty"`
	UserType       string            `json:"user_type,omitempty"`
	OrganizationID string            `json:"organization_id,omitempty"`
	DepartmentID   string            `json:"department_id,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	FromDate       *time.Time        `json:"from_date,omitempty"`
	ToDate         *time.Time        `json:"to_date,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	Offset         int               `json:"offset,omitempty"`
	SortBy         string            `json:"sort_by,omitempty"`
	SortOrder      string            `json:"sort_order,omitempty"`
}

type BelongsToDepartment struct {
	UserID       string `json:"user_id"`
	DepartmentID string `json:"department_id"`
}
