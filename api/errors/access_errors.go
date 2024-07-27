package errors

import "errors"

var (
	ErrRoleNotFound    = errors.New("role not found")
	ErrRoleConflict    = errors.New("role conflict")
	ErrInvalidRoleData = errors.New("invalid role data")

	ErrGroupNotFound    = errors.New("group not found")
	ErrGroupConflict    = errors.New("group conflict")
	ErrInvalidGroupData = errors.New("invalid group data")

	ErrPermissionNotFound    = errors.New("permission not found")
	ErrPermissionConflict    = errors.New("permission conflict")
	ErrInvalidPermissionData = errors.New("invalid permission data")
)
