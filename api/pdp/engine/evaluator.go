package engine

import (
	"context"
	"fmt"
	"time"

	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	pdp_model "github.com/dev-mohitbeniwal/echo/api/pdp/model"
	"go.uber.org/zap"
)

type PolicyEvaluator struct {
	cache *PolicyCache
}

func NewPolicyEvaluator(cacheSize int) *PolicyEvaluator {
	return &PolicyEvaluator{
		cache: NewPolicyCache(cacheSize),
	}
}

func (pe *PolicyEvaluator) Evaluate(ctx context.Context, request *pdp_model.AccessRequest, policies []*model.Policy) (*pdp_model.AccessDecision, error) {
	cacheKey := pe.generateCacheKey(request)
	if cachedDecision := pe.cache.Get(cacheKey); cachedDecision != nil {
		logger.Info("Cache hit for access request", zap.String("subject", request.Subject.ID), zap.String("resource", request.Resource.ID))
		return cachedDecision, nil
	}

	var decisions []pdp_model.PolicyEvaluationResult

	for _, policy := range policies {
		result := pe.evaluatePolicy(ctx, request, policy)
		decisions = append(decisions, result)
	}

	finalDecision := pe.combineDecisions(decisions)
	pe.cache.Set(cacheKey, finalDecision)

	return finalDecision, nil
}

func (pe *PolicyEvaluator) evaluatePolicy(ctx context.Context, request *pdp_model.AccessRequest, policy *model.Policy) pdp_model.PolicyEvaluationResult {
	result := pdp_model.PolicyEvaluationResult{
		PolicyID: policy.ID,
		Effect:   policy.Effect,
		Matched:  true,
		Priority: policy.Priority,
	}

	// Evaluate subjects
	subjectMatched := false
	for _, subject := range policy.Subjects {
		if pe.matchSubject(request.Subject, subject) {
			subjectMatched = true
			break
		}
	}
	if !subjectMatched {
		result.Matched = false
		result.Reason = "Subject did not match"
		return result
	}

	// Evaluate resource types
	resourceTypeMatched := false
	for _, resourceType := range policy.ResourceTypes {
		if resourceType == request.Resource.Type {
			resourceTypeMatched = true
			break
		}
	}
	if !resourceTypeMatched {
		result.Matched = false
		result.Reason = "Resource type did not match"
		return result
	}

	// Evaluate actions
	actionMatched := false
	for _, action := range policy.Actions {
		if action == request.Action {
			actionMatched = true
			break
		}
	}
	if !actionMatched {
		result.Matched = false
		result.Reason = "Action did not match"
		return result
	}

	// Evaluate conditions
	for _, condition := range policy.Conditions {
		if !pe.evaluateCondition(condition, request) {
			result.Matched = false
			result.Reason = fmt.Sprintf("Condition %s did not match", condition.Attribute)
			return result
		}
	}

	return result
}

func (pe *PolicyEvaluator) matchSubject(requestSubject pdp_model.Subject, policySubject model.Subject) bool {
	if policySubject.Type != requestSubject.Type {
		return false
	}

	if policySubject.Type == "user" && policySubject.UserID == requestSubject.ID {
		return true
	}

	// Add logic for matching roles and groups if needed

	return false
}

func (pe *PolicyEvaluator) evaluateCondition(condition model.Condition, request *pdp_model.AccessRequest) bool {
	switch condition.Attribute {
	case "time":
		return pe.evaluateTimeCondition(condition, request)
	case "ip_address":
		return pe.evaluateIPAddressCondition(condition, request)
	// Add more condition types as needed
	default:
		logger.Warn("Unknown condition type", zap.String("attribute", condition.Attribute))
		return false
	}
}

func (pe *PolicyEvaluator) evaluateTimeCondition(condition model.Condition, request *pdp_model.AccessRequest) bool {
	currentTime := time.Now()
	switch condition.Operator {
	case "between":
		startTime, _ := time.Parse(time.RFC3339, condition.Value.([]string)[0])
		endTime, _ := time.Parse(time.RFC3339, condition.Value.([]string)[1])
		return currentTime.After(startTime) && currentTime.Before(endTime)
	// Add more time-based operators as needed
	default:
		logger.Warn("Unknown time condition operator", zap.String("operator", condition.Operator))
		return false
	}
}

func (pe *PolicyEvaluator) evaluateIPAddressCondition(condition model.Condition, request *pdp_model.AccessRequest) bool {
	// Implement IP address condition evaluation
	// This is a placeholder and should be implemented based on your specific requirements
	return true
}

func (pe *PolicyEvaluator) combineDecisions(decisions []pdp_model.PolicyEvaluationResult) *pdp_model.AccessDecision {
	var highestPriorityAllow, highestPriorityDeny *pdp_model.PolicyEvaluationResult

	for i, decision := range decisions {
		if !decision.Matched {
			continue
		}

		if decision.Effect == "allow" {
			if highestPriorityAllow == nil || decision.Priority > highestPriorityAllow.Priority {
				highestPriorityAllow = &decisions[i]
			}
		} else if decision.Effect == "deny" {
			if highestPriorityDeny == nil || decision.Priority > highestPriorityDeny.Priority {
				highestPriorityDeny = &decisions[i]
			}
		}
	}

	if highestPriorityDeny != nil {
		return &pdp_model.AccessDecision{
			Effect: "deny",
			Reason: "Denied by highest priority deny policy",
		}
	}

	if highestPriorityAllow != nil {
		return &pdp_model.AccessDecision{
			Effect: "allow",
			Reason: "Allowed by highest priority allow policy",
		}
	}

	return &pdp_model.AccessDecision{
		Effect: "deny",
		Reason: "No matching policies found",
	}
}

func (pe *PolicyEvaluator) generateCacheKey(request *pdp_model.AccessRequest) string {
	return fmt.Sprintf("%s:%s:%s", request.Subject.ID, request.Resource.ID, request.Action)
}

type PolicyCache struct {
	cache map[string]*pdp_model.AccessDecision
	size  int
}

func NewPolicyCache(size int) *PolicyCache {
	return &PolicyCache{
		cache: make(map[string]*pdp_model.AccessDecision),
		size:  size,
	}
}

func (pc *PolicyCache) Get(key string) *pdp_model.AccessDecision {
	return pc.cache[key]
}

func (pc *PolicyCache) Set(key string, decision *pdp_model.AccessDecision) {
	if len(pc.cache) >= pc.size {
		// Implement cache eviction strategy (e.g., LRU) if needed
		for k := range pc.cache {
			delete(pc.cache, k)
			break
		}
	}
	pc.cache[key] = decision
}
