package model

import "time"

type PolicyEvaluationResult struct {
	PolicyID string
	Effect   string
	Matched  bool
	Reason   string
	Priority int
}

type ConditionEvaluationResult struct {
	ConditionID string
	Matched     bool
	Reason      string
}

type AttributeResolutionResult struct {
	AttributeName  string
	ResolvedValue  interface{}
	ResolutionTime time.Time
	Source         string // e.g., "user", "resource", "environment"
	Error          error
}
