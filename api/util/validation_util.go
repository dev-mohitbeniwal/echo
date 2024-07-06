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
	if len(policy.Resources) == 0 {
		return fmt.Errorf("policy must have at least one resource")
	}
	if len(policy.Actions) == 0 {
		return fmt.Errorf("policy must have at least one action")
	}
	// Add more validation rules as needed
	return nil
}
