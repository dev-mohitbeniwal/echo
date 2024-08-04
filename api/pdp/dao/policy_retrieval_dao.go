package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	pdp_model "github.com/dev-mohitbeniwal/echo/api/pdp/model"

	echo_neo4j "github.com/dev-mohitbeniwal/echo/api/model/neo4j"
)

type PolicyRetrievalDAO struct {
	Driver neo4j.Driver
}

func NewPolicyRetrievalDAO(driver neo4j.Driver) *PolicyRetrievalDAO {
	return &PolicyRetrievalDAO{Driver: driver}
}

func (dao *PolicyRetrievalDAO) RetrieveRelevantPolicies(ctx context.Context, request *pdp_model.AccessRequest) ([]*model.Policy, error) {
	start := time.Now()
	logger.Info("Retrieving relevant policies for access request",
		zap.String("subject_id", request.Subject.ID),
		zap.String("resource_id", request.Resource.ID),
		zap.String("action", request.Action))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (s:` + echo_neo4j.LabelUser + ` {id: $subjectID})
        MATCH (r:` + echo_neo4j.LabelResource + ` {id: $resourceID})
        MATCH (p:` + echo_neo4j.LabelPolicy + `)
        WHERE 
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(s) OR
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(:` + echo_neo4j.LabelRole + `)<-[:` + echo_neo4j.RelHasRole + `]-(s) OR
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(:` + echo_neo4j.LabelGroup + `)<-[:` + echo_neo4j.RelBelongsToGroup + `]-(s)
        AND
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(r) OR
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(:` + echo_neo4j.LabelResourceType + `)<-[:` + echo_neo4j.RelHasType + `]-(r) OR
            (p)-[:` + echo_neo4j.RelAppliesTo + `]->(:` + echo_neo4j.LabelAttributeGroup + `)<-[:` + echo_neo4j.RelInAttributeGroup + `]-(r)
        AND
            $action IN p.actions
        AND
            p.active = true
        AND
            (p.activationDate IS NULL OR p.activationDate <= datetime())
        AND
            (p.deactivationDate IS NULL OR p.deactivationDate > datetime())
        RETURN p
        ORDER BY p.priority DESC
        `

		params := map[string]interface{}{
			"subjectID":  request.Subject.ID,
			"resourceID": request.Resource.ID,
			"action":     request.Action,
		}

		result, err := tx.Run(query, params)
		if err != nil {
			return nil, err
		}

		var policies []*model.Policy
		for result.Next() {
			record := result.Record()
			policyNode := record.Values[0].(neo4j.Node)
			policy, err := mapNodeToPolicy(policyNode)
			if err != nil {
				return nil, err
			}
			policies = append(policies, policy)
		}

		return policies, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to retrieve relevant policies",
			zap.Error(err),
			zap.Duration("duration", duration))
		return nil, err
	}

	policies := result.([]*model.Policy)
	logger.Info("Retrieved relevant policies successfully",
		zap.Int("policy_count", len(policies)),
		zap.Duration("duration", duration))

	return policies, nil
}

// Helper function to map Neo4j Node to Policy struct
func mapNodeToPolicy(node neo4j.Node) (*model.Policy, error) {
	props := node.Props
	policy := &model.Policy{}

	// ID
	if id, ok := props["id"].(string); ok {
		policy.ID = id
	} else {
		return nil, fmt.Errorf("failed to assert type for policy ID: %v", props["id"])
	}

	// Name
	if name, ok := props["name"].(string); ok {
		policy.Name = name
	} else {
		return nil, fmt.Errorf("failed to assert type for policy name: %v", props["name"])
	}

	// Description
	if description, ok := props["description"].(string); ok {
		policy.Description = description
	} else {
		return nil, fmt.Errorf("failed to assert type for policy description: %v", props["description"])
	}

	// Effect
	if effect, ok := props["effect"].(string); ok {
		if effect == echo_neo4j.PolicyEffectAllow || effect == echo_neo4j.PolicyEffectDeny {
			policy.Effect = effect
		} else {
			return nil, fmt.Errorf("invalid policy effect: %v", effect)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy effect: %v", props["effect"])
	}

	// Priority
	if priority, ok := props["priority"].(int64); ok {
		policy.Priority = int(priority)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy priority: %v", props["priority"])
	}

	// Version
	if version, ok := props["version"].(int64); ok {
		policy.Version = int(version)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy version: %v", props["version"])
	}

	// ParentPolicyID
	if parentPolicyID, ok := props["parentPolicyID"].(string); ok {
		policy.ParentPolicyID = parentPolicyID
	} else {
		logger.Warn("Parent policy ID not found or null", zap.Any("ParentPolicyID", props["parentPolicyID"]))
	}

	// CreatedAt
	if createdAt, ok := props["createdAt"].(string); ok {
		policy.CreatedAt, _ = helper_util.ParseTime(createdAt)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy createdAt: %v", props["createdAt"])
	}

	// UpdatedAt
	if updatedAt, ok := props["updatedAt"].(string); ok {
		policy.UpdatedAt, _ = helper_util.ParseTime(updatedAt)
	} else {
		return nil, fmt.Errorf("failed to assert type for policy updatedAt: %v", props["updatedAt"])
	}

	// Active
	if active, ok := props["active"].(bool); ok {
		policy.Active = active
	} else {
		return nil, fmt.Errorf("failed to assert type for policy active: %v", props["active"])
	}

	// ActivationDate
	if activationDate, ok := props["activationDate"]; ok {
		policy.ActivationDate, _ = helper_util.ParseNullableTime(activationDate)
	} else {
		logger.Warn("Activation date not found or null", zap.Any("ActivationDate", props["activationDate"]))
	}

	// DeactivationDate
	if deactivationDate, ok := props["deactivationDate"]; ok {
		policy.DeactivationDate, _ = helper_util.ParseNullableTime(deactivationDate)
	} else {
		logger.Warn("Deactivation date not found or null", zap.Any("DeactivationDate", props["deactivationDate"]))
	}

	// Subjects
	if subjectsJSON, ok := props["subjects"].(string); ok {
		if err := json.Unmarshal([]byte(subjectsJSON), &policy.Subjects); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy subjects: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy subjects: %v", props["subjects"])
	}

	// ResourceTypes
	if resourceTypesJSON, ok := props["resourceTypes"].(string); ok {
		if err := json.Unmarshal([]byte(resourceTypesJSON), &policy.ResourceTypes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy resource types: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy resource types: %v", props["resourceTypes"])
	}

	// AttributeGroups
	if attributeGroupsJSON, ok := props["attributeGroups"].(string); ok {
		if err := json.Unmarshal([]byte(attributeGroupsJSON), &policy.AttributeGroups); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy attribute groups: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy attribute groups: %v", props["attributeGroups"])
	}

	// Actions
	if actionsJSON, ok := props["actions"].(string); ok {
		if err := json.Unmarshal([]byte(actionsJSON), &policy.Actions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy actions: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy actions: %v", props["actions"])
	}

	// Conditions
	if conditionsJSON, ok := props["conditions"].(string); ok {
		if err := json.Unmarshal([]byte(conditionsJSON), &policy.Conditions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy conditions: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to assert type for policy conditions: %v", props["conditions"])
	}

	// DynamicAttributes
	if dynamicAttributesJSON, ok := props["dynamicAttributes"].(string); ok {
		if err := json.Unmarshal([]byte(dynamicAttributesJSON), &policy.DynamicAttributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy dynamic attributes: %w", err)
		}
	} else {
		logger.Warn("Dynamic attributes not found or null", zap.Any("DynamicAttributes", props["dynamicAttributes"]))
	}

	return policy, nil
}
