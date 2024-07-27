// api/controller/group_controller.go
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

type GroupController struct {
	groupService service.IGroupService
}

func NewGroupController(groupService service.IGroupService) *GroupController {
	return &GroupController{
		groupService: groupService,
	}
}

// RegisterRoutes registers the API routes for groups
func (gc *GroupController) RegisterRoutes(r *gin.RouterGroup) {
	groups := r.Group("/groups")
	{
		groups.POST("", gc.CreateGroup)
		groups.PUT("/:id", gc.UpdateGroup)
		groups.DELETE("/:id", gc.DeleteGroup)
		groups.GET("/:id", gc.GetGroup)
		groups.GET("", gc.ListGroups)
		groups.GET("/search", gc.SearchGroups)
	}
}

// CreateGroup endpoint
func (gc *GroupController) CreateGroup(c *gin.Context) {
	var group model.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid group data", echo_errors.ErrInvalidGroupData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdGroup, err := gc.groupService.CreateGroup(c, group, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrGroupConflict:
			util.RespondWithError(c, http.StatusConflict, "Group already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create group", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdGroup)
}

// UpdateGroup endpoint
func (gc *GroupController) UpdateGroup(c *gin.Context) {
	groupID := c.Param("id")
	var group model.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid group data", err)
		return
	}
	group.ID = groupID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedGroup, err := gc.groupService.UpdateGroup(c, group, updaterID)
	if err != nil {
		if err == echo_errors.ErrGroupNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update group", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedGroup)
}

// DeleteGroup endpoint
func (gc *GroupController) DeleteGroup(c *gin.Context) {
	groupID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := gc.groupService.DeleteGroup(c, groupID, deleterID); err != nil {
		if err == echo_errors.ErrGroupNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete group", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetGroup endpoint
func (gc *GroupController) GetGroup(c *gin.Context) {
	groupID := c.Param("id")

	group, err := gc.groupService.GetGroup(c, groupID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrGroupNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Group not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve group", err)
		}
		return
	}

	c.JSON(http.StatusOK, group)
}

// ListGroups endpoint
func (gc *GroupController) ListGroups(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	groups, err := gc.groupService.ListGroups(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list groups", err)
		return
	}

	c.JSON(http.StatusOK, groups)
}

// SearchGroups endpoint
func (gc *GroupController) SearchGroups(c *gin.Context) {
	query := c.Query("query")

	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	if query == "" {
		util.RespondWithError(c, http.StatusBadRequest, "Query parameter is required", nil)
		return
	}

	groups, err := gc.groupService.SearchGroups(c, query, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search groups", err)
		return
	}

	c.JSON(http.StatusOK, groups)
}
