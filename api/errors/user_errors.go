// api/errors/user_errors.go
package errors

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidUserData = errors.New("invalid user data")
	ErrUserConflict    = errors.New("user conflict")
)
