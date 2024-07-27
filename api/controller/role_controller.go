// api/controller/role_controller.go
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

type RoleController struct {
	roleService service.IRoleService
}

func NewRoleController(roleService service.IRoleService) *RoleController {
	return &RoleController{
		roleService: roleService,
	}
}

// RegisterRoutes registers the API routes for roles
func (rc *RoleController) RegisterRoutes(r *gin.RouterGroup) {
	roles := r.Group("/roles")
	{
		roles.POST("", rc.CreateRole)
		roles.PUT("/:id", rc.UpdateRole)
		roles.DELETE("/:id", rc.DeleteRole)
		roles.GET("/:id", rc.GetRole)
		roles.GET("", rc.ListRoles)
		roles.GET("/search", rc.SearchRoles)
	}
}

// CreateRole endpoint
func (rc *RoleController) CreateRole(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid role data", echo_errors.ErrInvalidRoleData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdRole, err := rc.roleService.CreateRole(c, role, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrRoleConflict:
			util.RespondWithError(c, http.StatusConflict, "Role already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create role", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdRole)
}

// UpdateRole endpoint
func (rc *RoleController) UpdateRole(c *gin.Context) {
	roleID := c.Param("id")
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid role data", err)
		return
	}
	role.ID = roleID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedRole, err := rc.roleService.UpdateRole(c, role, updaterID)
	if err != nil {
		if err == echo_errors.ErrRoleNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Role not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update role", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedRole)
}

// DeleteRole endpoint
func (rc *RoleController) DeleteRole(c *gin.Context) {
	roleID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := rc.roleService.DeleteRole(c, roleID, deleterID); err != nil {
		if err == echo_errors.ErrRoleNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Role not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete role", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetRole endpoint
func (rc *RoleController) GetRole(c *gin.Context) {
	roleID := c.Param("id")

	role, err := rc.roleService.GetRole(c, roleID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrRoleNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Role not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve role", err)
		}
		return
	}

	c.JSON(http.StatusOK, role)
}

// ListRoles endpoint
func (rc *RoleController) ListRoles(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	roles, err := rc.roleService.ListRoles(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list roles", err)
		return
	}

	c.JSON(http.StatusOK, roles)
}

// SearchRoles endpoint
func (rc *RoleController) SearchRoles(c *gin.Context) {
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

	roles, err := rc.roleService.SearchRoles(c, query, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search roles", err)
		return
	}

	c.JSON(http.StatusOK, roles)
}
