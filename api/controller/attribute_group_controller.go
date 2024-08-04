// api/controller/attribute_group_controller.go
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

type AttributeGroupController struct {
	attributeGroupService service.IAttributeGroupService
}

func NewAttributeGroupController(attributeGroupService service.IAttributeGroupService) *AttributeGroupController {
	return &AttributeGroupController{
		attributeGroupService: attributeGroupService,
	}
}

// RegisterRoutes registers the API routes for attribute group management
func (agc *AttributeGroupController) RegisterRoutes(r *gin.RouterGroup) {
	attributeGroups := r.Group("/attribute-groups")
	{
		attributeGroups.POST("", agc.CreateAttributeGroup)
		attributeGroups.PUT("/:id", agc.UpdateAttributeGroup)
		attributeGroups.DELETE("/:id", agc.DeleteAttributeGroup)
		attributeGroups.GET("/:id", agc.GetAttributeGroup)
		attributeGroups.GET("", agc.ListAttributeGroups)
	}
}

// CreateAttributeGroup endpoint
func (agc *AttributeGroupController) CreateAttributeGroup(c *gin.Context) {
	var attributeGroup model.AttributeGroup
	if err := c.ShouldBindJSON(&attributeGroup); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid attribute group data", echo_errors.ErrInvalidAttributeGroupData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdAttributeGroup, err := agc.attributeGroupService.CreateAttributeGroup(c, attributeGroup, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrAttributeGroupConflict:
			util.RespondWithError(c, http.StatusConflict, "Attribute group already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create attribute group", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdAttributeGroup)
}

// UpdateAttributeGroup endpoint
func (agc *AttributeGroupController) UpdateAttributeGroup(c *gin.Context) {
	attributeGroupID := c.Param("id")
	var attributeGroup model.AttributeGroup
	if err := c.ShouldBindJSON(&attributeGroup); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid attribute group data", err)
		return
	}
	attributeGroup.ID = attributeGroupID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedAttributeGroup, err := agc.attributeGroupService.UpdateAttributeGroup(c, attributeGroup, updaterID)
	if err != nil {
		if err == echo_errors.ErrAttributeGroupNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Attribute group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update attribute group", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedAttributeGroup)
}

// DeleteAttributeGroup endpoint
func (agc *AttributeGroupController) DeleteAttributeGroup(c *gin.Context) {
	attributeGroupID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := agc.attributeGroupService.DeleteAttributeGroup(c, attributeGroupID, deleterID); err != nil {
		if err == echo_errors.ErrAttributeGroupNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Attribute group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete attribute group", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAttributeGroup endpoint
func (agc *AttributeGroupController) GetAttributeGroup(c *gin.Context) {
	attributeGroupID := c.Param("id")

	attributeGroup, err := agc.attributeGroupService.GetAttributeGroup(c, attributeGroupID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrAttributeGroupNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Attribute group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve attribute group", err)
		}
		return
	}

	c.JSON(http.StatusOK, attributeGroup)
}

// ListAttributeGroups endpoint
func (agc *AttributeGroupController) ListAttributeGroups(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	attributeGroups, err := agc.attributeGroupService.ListAttributeGroups(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list attribute groups", err)
		return
	}

	c.JSON(http.StatusOK, attributeGroups)
}
