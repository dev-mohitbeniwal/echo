// api/errors/policy_errors.go
package errors

import "errors"

var (
	ErrPolicyNotFound        = errors.New("policy not found")
	ErrDatabaseOperation     = errors.New("database operation failed")
	ErrInvalidPolicyData     = errors.New("invalid policy data")
	ErrPolicyConflict        = errors.New("policy conflict")
	ErrInternalServer        = errors.New("internal server error")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidPagination     = errors.New("invalid pagination parameters")
	ErrInvalidSearchCriteria = errors.New("invalid search criteria")
)
