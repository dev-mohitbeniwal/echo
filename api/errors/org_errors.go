// api/errors/policy_errors.go
package errors

import "errors"

var (
	ErrOrganizationNotFound    = errors.New("organization not found")
	ErrDepartmentNotFound      = errors.New("department not found")
	ErrOrganizationConflict    = errors.New("organization conflict")
	ErrInvalidOrganizationData = errors.New("invalid organization data")
	ErrDepartmentConflict      = errors.New("department conflict")
	ErrInvalidDepartmentData   = errors.New("invalid department data")
)
