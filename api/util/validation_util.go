// api/util/validation_util.go

package util

import (
	"fmt"

	"github.com/dev-mohitbeniwal/echo/api/model"
)

type ValidationUtil struct{}

func NewValidationUtil() *ValidationUtil {
	return &ValidationUtil{}
}

func (v *ValidationUtil) ValidatePolicy(policy model.Policy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name cannot be empty")
	}
	if policy.Effect != "allow" && policy.Effect != "deny" {
		return fmt.Errorf("policy effect must be either 'allow' or 'deny'")
	}
	if policy.Priority < 0 {
		return fmt.Errorf("policy priority cannot be negative")
	}
	if len(policy.Subjects) == 0 {
		return fmt.Errorf("policy must have at least one subject")
	}
	if len(policy.ResourceTypes) == 0 {
		return fmt.Errorf("policy must have at least one resource")
	}
	if len(policy.Actions) == 0 {
		return fmt.Errorf("policy must have at least one action")
	}
	// Add more validation rules as needed
	return nil
}

func (v *ValidationUtil) ValidateOrganization(organization model.Organization) error {
	if organization.ID == "" {
		return fmt.Errorf("organization description cannot be empty")
	}
	if organization.Name == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

func (v *ValidationUtil) ValidateDepartment(department model.Department) error {
	if department.ID == "" {
		return fmt.Errorf("department ID cannot be empty")
	}
	if department.Name == "" {
		return fmt.Errorf("department name cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

func (v *ValidationUtil) ValidateUser(user model.User) error {
	if user.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if user.Name == "" {
		return fmt.Errorf("user name cannot be empty")
	}
	if user.Username == "" {
		return fmt.Errorf("user username cannot be empty")
	}
	if user.Email == "" {
		return fmt.Errorf("user email cannot be empty")
	}
	if user.UserType == "" {
		return fmt.Errorf("user type cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

func (v *ValidationUtil) ValidateRole(role model.Role) error {
	if role.ID == "" {
		return fmt.Errorf("role ID cannot be empty")
	}
	if role.Name == "" {
		return fmt.Errorf("role name cannot be empty")
	}
	if role.OrganizationID == "" {
		return fmt.Errorf("role organization ID cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

func (v *ValidationUtil) ValidateGroup(group model.Group) error {
	if group.ID == "" {
		return fmt.Errorf("group ID cannot be empty")
	}
	if group.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	if group.OrganizationID == "" {
		return fmt.Errorf("group organization ID cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

// ValidatePermission
func (v *ValidationUtil) ValidatePermission(permission model.Permission) error {
	if permission.ID == "" {
		return fmt.Errorf("permission ID cannot be empty")
	}
	if permission.Name == "" {
		return fmt.Errorf("permission name cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

// ValidateResource
func (v *ValidationUtil) ValidateResource(resource model.Resource) error {
	if resource.ID == "" {
		return fmt.Errorf("resource ID cannot be empty")
	}
	if resource.Name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if resource.Type == "" {
		return fmt.Errorf("resource type cannot be empty")
	}
	if resource.OrganizationID == "" {
		return fmt.Errorf("resource organization ID cannot be empty")
	}
	if resource.OwnerID == "" {
		return fmt.Errorf("resource owner ID cannot be empty")
	}
	if resource.Status == "" {
		return fmt.Errorf("resource status cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

// ValidateResourceType
func (v *ValidationUtil) ValidateResourceType(resourceType model.ResourceType) error {
	if resourceType.ID == "" {
		return fmt.Errorf("resource type ID cannot be empty")
	}
	if resourceType.Name == "" {
		return fmt.Errorf("resource type name cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}

// ValidateAttributeGroup
func (v *ValidationUtil) ValidateAttributeGroup(attributeGroup model.AttributeGroup) error {
	if attributeGroup.ID == "" {
		return fmt.Errorf("attribute group ID cannot be empty")
	}
	if attributeGroup.Name == "" {
		return fmt.Errorf("attribute group name cannot be empty")
	}
	// Add more validation rules as needed
	return nil
}
