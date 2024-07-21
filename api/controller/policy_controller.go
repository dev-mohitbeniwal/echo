// api/controller/policy_controller.go
package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	"github.com/dev-mohitbeniwal/echo/api/model"
	"github.com/dev-mohitbeniwal/echo/api/service"
	"github.com/dev-mohitbeniwal/echo/api/util"
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
)

type PolicyController struct {
	policyService service.IPolicyService
}

func NewPolicyController(policyService service.IPolicyService) *PolicyController {
	return &PolicyController{
		policyService: policyService,
	}
}

// RegisterRoutes registers the API routes
func (pc *PolicyController) RegisterRoutes(r *gin.RouterGroup) {
	policies := r.Group("/policies")
	{
		policies.POST("", pc.CreatePolicy)
		policies.PUT("/:id", pc.UpdatePolicy)
		policies.DELETE("/:id", pc.DeletePolicy)
		policies.GET("/:id", pc.GetPolicy)
		policies.GET("", pc.ListPolicies)
		policies.POST("/search", pc.SearchPolicies)
		policies.GET("/:id/usage", pc.AnalyzePolicyUsage)
	}
}

// CreatePolicy endpoint
func (pc *PolicyController) CreatePolicy(c *gin.Context) {
	var policy model.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid policy data", echo_errors.ErrInvalidPolicyData)
		return
	}
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdPolicy, err := pc.policyService.CreatePolicy(c, policy, userID)
	if err != nil {
		switch err {
		case echo_errors.ErrPolicyConflict:
			util.RespondWithError(c, http.StatusConflict, "Policy already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create policy", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdPolicy)
}

// UpdatePolicy endpoint
func (pc *PolicyController) UpdatePolicy(c *gin.Context) {
	policyID := c.Param("id")
	var policy model.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid policy data", err)
		return
	}
	policy.ID = policyID
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedPolicy, err := pc.policyService.UpdatePolicy(c, policy, userID)
	if err != nil {
		if err == echo_errors.ErrPolicyNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Policy not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update policy", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedPolicy)
}

// DeletePolicy endpoint
func (pc *PolicyController) DeletePolicy(c *gin.Context) {
	policyID := c.Param("id")
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := pc.policyService.DeletePolicy(c, policyID, userID); err != nil {
		if err == echo_errors.ErrPolicyNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Policy not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete policy", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPolicy endpoint
func (pc *PolicyController) GetPolicy(c *gin.Context) {
	policyID := c.Param("id")

	policy, err := pc.policyService.GetPolicy(c, policyID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrPolicyNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Policy not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve policy", err)
		}
		return
	}

	c.JSON(http.StatusOK, policy)
}

// ListPolicies endpoint
func (pc *PolicyController) ListPolicies(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	policies, err := pc.policyService.ListPolicies(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list policies", err)
		return
	}

	c.JSON(http.StatusOK, policies)
}

// SearchPolicies endpoint
func (pc *PolicyController) SearchPolicies(c *gin.Context) {
	var criteria model.PolicySearchCriteria
	if err := c.ShouldBindJSON(&criteria); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid search criteria", err)
		return
	}

	policies, err := pc.policyService.SearchPolicies(c, criteria)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search policies", err)
		return
	}

	c.JSON(http.StatusOK, policies)
}

// AnalyzePolicyUsage endpoint
func (pc *PolicyController) AnalyzePolicyUsage(c *gin.Context) {
	policyID := c.Param("id")

	analysis, err := pc.policyService.AnalyzePolicyUsage(c, policyID)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to analyze policy usage", err)
		return
	}

	c.JSON(http.StatusOK, analysis)
}
