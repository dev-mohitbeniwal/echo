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

type BelongsToDepartment struct {
	UserID       string `json:"user_id"`
	DepartmentID string `json:"department_id"`
}
