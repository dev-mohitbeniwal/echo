// api/controller/resource_type_controller.go
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

type ResourceTypeController struct {
	resourceTypeService service.IResourceTypeService
}

func NewResourceTypeController(resourceTypeService service.IResourceTypeService) *ResourceTypeController {
	return &ResourceTypeController{
		resourceTypeService: resourceTypeService,
	}
}

// RegisterRoutes registers the API routes for resource type management
func (rtc *ResourceTypeController) RegisterRoutes(r *gin.RouterGroup) {
	resourceTypes := r.Group("/resource-types")
	{
		resourceTypes.POST("", rtc.CreateResourceType)
		resourceTypes.PUT("/:id", rtc.UpdateResourceType)
		resourceTypes.DELETE("/:id", rtc.DeleteResourceType)
		resourceTypes.GET("/:id", rtc.GetResourceType)
		resourceTypes.GET("", rtc.ListResourceTypes)
	}
}

// CreateResourceType endpoint
func (rtc *ResourceTypeController) CreateResourceType(c *gin.Context) {
	var resourceType model.ResourceType
	if err := c.ShouldBindJSON(&resourceType); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid resource type data", echo_errors.ErrInvalidResourceTypeData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdResourceType, err := rtc.resourceTypeService.CreateResourceType(c, resourceType, creatorID)
	if err != nil {
		switch {
		case errors.Is(err, echo_errors.ErrInvalidResourceType):
			util.RespondWithError(c, http.StatusBadRequest, "Invalid resource type", err)
		case errors.Is(err, echo_errors.ErrDatabaseOperation):
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create resource type", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdResourceType)
}

// UpdateResourceType endpoint
func (rtc *ResourceTypeController) UpdateResourceType(c *gin.Context) {
	resourceTypeID := c.Param("id")
	var resourceType model.ResourceType
	if err := c.ShouldBindJSON(&resourceType); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid resource type data", err)
		return
	}
	resourceType.ID = resourceTypeID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedResourceType, err := rtc.resourceTypeService.UpdateResourceType(c, resourceType, updaterID)
	if err != nil {
		switch {
		case errors.Is(err, echo_errors.ErrResourceTypeNotFound):
			util.RespondWithError(c, http.StatusNotFound, "Resource type not found", err)
		case errors.Is(err, echo_errors.ErrInvalidResourceType):
			util.RespondWithError(c, http.StatusBadRequest, "Invalid resource type", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update resource type", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedResourceType)
}

// DeleteResourceType endpoint
func (rtc *ResourceTypeController) DeleteResourceType(c *gin.Context) {
	resourceTypeID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := rtc.resourceTypeService.DeleteResourceType(c, resourceTypeID, deleterID); err != nil {
		switch {
		case errors.Is(err, echo_errors.ErrResourceTypeNotFound):
			util.RespondWithError(c, http.StatusNotFound, "Resource type not found", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete resource type", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetResourceType endpoint
func (rtc *ResourceTypeController) GetResourceType(c *gin.Context) {
	resourceTypeID := c.Param("id")

	resourceType, err := rtc.resourceTypeService.GetResourceType(c, resourceTypeID)
	if err != nil {
		switch {
		case errors.Is(err, echo_errors.ErrResourceTypeNotFound):
			util.RespondWithError(c, http.StatusNotFound, "Resource type not found", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve resource type", err)
		}
		return
	}

	c.JSON(http.StatusOK, resourceType)
}

// ListResourceTypes endpoint
func (rtc *ResourceTypeController) ListResourceTypes(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	resourceTypes, err := rtc.resourceTypeService.ListResourceTypes(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list resource types", err)
		return
	}

	c.JSON(http.StatusOK, resourceTypes)
}
