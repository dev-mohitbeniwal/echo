package model

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationSearchCriteria struct {
	Name      string     `json:"name,omitempty"`
	ID        string     `json:"id,omitempty"`
	FromDate  *time.Time `json:"from_date,omitempty"`
	ToDate    *time.Time `json:"to_date,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
	SortBy    string     `json:"sort_by,omitempty"`
	SortOrder string     `json:"sort_order,omitempty"`
}

type Department struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"organization_id"`
	ParentID       string    `json:"parent_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DepartmentSearchCriteria struct {
	ID             string     `json:"id,omitempty"`
	Name           string     `json:"name,omitempty"`
	OrganizationID string     `json:"organization_id,omitempty"`
	ParentID       string     `json:"parent_id,omitempty"`
	FromDate       *time.Time `json:"from_date,omitempty"`
	ToDate         *time.Time `json:"to_date,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
	SortBy         string     `json:"sort_by,omitempty"`
	SortOrder      string     `json:"sort_order,omitempty"`
}
