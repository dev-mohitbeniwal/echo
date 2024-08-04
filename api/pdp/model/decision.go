package model

import "time"

type AccessDecision struct {
	Effect            string                 `json:"effect"`
	Status            string                 `json:"status"`
	Reason            string                 `json:"reason,omitempty"`
	Obligations       []Obligation           `json:"obligations,omitempty"`
	Advice            map[string]interface{} `json:"advice,omitempty"`
	EvaluatedPolicies []string               `json:"evaluated_policies,omitempty"`
}

type Obligation struct {
	Type       string                 `json:"type"`
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type DecisionCacheEntry struct {
	Decision  AccessDecision
	ExpiresAt time.Time
}
