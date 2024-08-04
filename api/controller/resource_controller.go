// api/controller/resource_controller.go
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

type ResourceController struct {
	resourceService service.IResourceService
}

func NewResourceController(resourceService service.IResourceService) *ResourceController {
	return &ResourceController{
		resourceService: resourceService,
	}
}

// RegisterRoutes registers the API routes for resource management
func (rc *ResourceController) RegisterRoutes(r *gin.RouterGroup) {
	resources := r.Group("/resources")
	{
		resources.POST("", rc.CreateResource)
		resources.PUT("/:id", rc.UpdateResource)
		resources.DELETE("/:id", rc.DeleteResource)
		resources.GET("/:id", rc.GetResource)
		resources.GET("", rc.ListResources)
		resources.POST("/search", rc.SearchResources)
	}
}

// CreateResource endpoint
func (rc *ResourceController) CreateResource(c *gin.Context) {
	var resource model.Resource
	if err := c.ShouldBindJSON(&resource); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid resource data", echo_errors.ErrInvalidResourceData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdResource, err := rc.resourceService.CreateResource(c, resource, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrResourceConflict:
			util.RespondWithError(c, http.StatusConflict, "Resource already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create resource", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdResource)
}

// UpdateResource endpoint
func (rc *ResourceController) UpdateResource(c *gin.Context) {
	resourceID := c.Param("id")
	var resource model.Resource
	if err := c.ShouldBindJSON(&resource); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid resource data", err)
		return
	}
	resource.ID = resourceID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedResource, err := rc.resourceService.UpdateResource(c, resource, updaterID)
	if err != nil {
		if err == echo_errors.ErrResourceNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Resource not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update resource", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedResource)
}

// DeleteResource endpoint
func (rc *ResourceController) DeleteResource(c *gin.Context) {
	resourceID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := rc.resourceService.DeleteResource(c, resourceID, deleterID); err != nil {
		if err == echo_errors.ErrResourceNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Resource not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete resource", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetResource endpoint
func (rc *ResourceController) GetResource(c *gin.Context) {
	resourceID := c.Param("id")

	resource, err := rc.resourceService.GetResource(c, resourceID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrResourceNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Resource not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve resource", err)
		}
		return
	}

	c.JSON(http.StatusOK, resource)
}

// ListResources endpoint
func (rc *ResourceController) ListResources(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	resources, err := rc.resourceService.ListResources(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list resources", err)
		return
	}

	c.JSON(http.StatusOK, resources)
}

// SearchResources endpoint
func (rc *ResourceController) SearchResources(c *gin.Context) {
	var criteria model.ResourceSearchCriteria

	if err := c.ShouldBindJSON(&criteria); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid search criteria", err)
		return
	}

	resources, err := rc.resourceService.SearchResources(c, criteria)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search resources", err)
		return
	}

	c.JSON(http.StatusOK, resources)
}
