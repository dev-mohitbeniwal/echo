// api/errors/resource_errors.go

package errors

import "errors"

var (
	ErrResourceNotFound          = errors.New("resource not found")
	ErrInvalidResourceData       = errors.New("invalid resource data")
	ErrResourceConflict          = errors.New("resource conflict")
	ErrResourceTypeNotFound      = errors.New("resource type not found")
	ErrAttributeGroupNotFound    = errors.New("attribute group not found")
	ErrAttributeGroupConflict    = errors.New("attribute group conflict")
	ErrInvalidAttributeGroupData = errors.New("invalid attribute group data")
	ErrInvalidResourceType       = errors.New("invalid resource type")
	ErrInvalidResourceTypeData   = errors.New("invalid resource type data")
)
