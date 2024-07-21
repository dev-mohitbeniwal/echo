// api/controller/organization_controller.go
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

type OrganizationController struct {
	organizationService service.IOrganizationService
}

func NewOrganizationController(organizationService service.IOrganizationService) *OrganizationController {
	return &OrganizationController{
		organizationService: organizationService,
	}
}

// RegisterRoutes registers the API routes
func (oc *OrganizationController) RegisterRoutes(r *gin.RouterGroup) {
	organizations := r.Group("/organizations")
	{
		organizations.POST("", oc.CreateOrganization)
		organizations.PUT("/:id", oc.UpdateOrganization)
		organizations.DELETE("/:id", oc.DeleteOrganization)
		organizations.GET("/:id", oc.GetOrganization)
		organizations.GET("", oc.ListOrganizations)
		organizations.GET("/search", oc.SearchOrganizations)
	}
}

// CreateOrganization endpoint
func (oc *OrganizationController) CreateOrganization(c *gin.Context) {
	var org model.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid organization data", echo_errors.ErrInvalidOrganizationData)
		return
	}
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdOrg, err := oc.organizationService.CreateOrganization(c, org, userID)
	if err != nil {
		switch err {
		case echo_errors.ErrOrganizationConflict:
			util.RespondWithError(c, http.StatusConflict, "Organization already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create organization", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdOrg)
}

// UpdateOrganization endpoint
func (oc *OrganizationController) UpdateOrganization(c *gin.Context) {
	orgID := c.Param("id")
	var org model.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid organization data", err)
		return
	}
	org.ID = orgID
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedOrg, err := oc.organizationService.UpdateOrganization(c, org, userID)
	if err != nil {
		if err == echo_errors.ErrOrganizationNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Organization not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update organization", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedOrg)
}

// DeleteOrganization endpoint
func (oc *OrganizationController) DeleteOrganization(c *gin.Context) {
	orgID := c.Param("id")
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := oc.organizationService.DeleteOrganization(c, orgID, userID); err != nil {
		if err == echo_errors.ErrOrganizationNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Organization not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete organization", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetOrganization endpoint
func (oc *OrganizationController) GetOrganization(c *gin.Context) {
	orgID := c.Param("id")

	org, err := oc.organizationService.GetOrganization(c, orgID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrOrganizationNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Organization not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve organization", err)
		}
		return
	}

	c.JSON(http.StatusOK, org)
}

// ListOrganizations endpoint
func (oc *OrganizationController) ListOrganizations(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	orgs, err := oc.organizationService.ListOrganizations(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list organizations", err)
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// SearchOrganizations endpoint
// SearchOrganizations endpoint
func (oc *OrganizationController) SearchOrganizations(c *gin.Context) {
	var criteria model.OrganizationSearchCriteria
	if err := c.ShouldBindJSON(&criteria); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid search criteria", err)
		return
	}

	orgs, err := oc.organizationService.SearchOrganizations(c, criteria)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search organizations", err)
		return
	}

	c.JSON(http.StatusOK, orgs)
}
