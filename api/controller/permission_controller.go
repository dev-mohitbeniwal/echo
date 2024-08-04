// api/controller/permission_controller.go
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

type PermissionController struct {
	permissionService service.IPermissionService
}

func NewPermissionController(permissionService service.IPermissionService) *PermissionController {
	return &PermissionController{
		permissionService: permissionService,
	}
}

// RegisterRoutes registers the API routes for permissions
func (pc *PermissionController) RegisterRoutes(r *gin.RouterGroup) {
	permissions := r.Group("/permissions")
	{
		permissions.POST("", pc.CreatePermission)
		permissions.PUT("/:id", pc.UpdatePermission)
		permissions.DELETE("/:id", pc.DeletePermission)
		permissions.GET("/:id", pc.GetPermission)
		permissions.GET("", pc.ListPermissions)
		permissions.GET("/search", pc.SearchPermissions)
	}
}

// CreatePermission endpoint
func (pc *PermissionController) CreatePermission(c *gin.Context) {
	var permission model.Permission
	if err := c.ShouldBindJSON(&permission); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid permission data", echo_errors.ErrInvalidPermissionData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdPermission, err := pc.permissionService.CreatePermission(c, permission, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrPermissionConflict:
			util.RespondWithError(c, http.StatusConflict, "Permission already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create permission", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdPermission)
}

// UpdatePermission endpoint
func (pc *PermissionController) UpdatePermission(c *gin.Context) {
	permissionID := c.Param("id")
	var permission model.Permission
	if err := c.ShouldBindJSON(&permission); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid permission data", err)
		return
	}
	permission.ID = permissionID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedPermission, err := pc.permissionService.UpdatePermission(c, permission, updaterID)
	if err != nil {
		if err == echo_errors.ErrPermissionNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Permission not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update permission", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedPermission)
}

// DeletePermission endpoint
func (pc *PermissionController) DeletePermission(c *gin.Context) {
	permissionID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := pc.permissionService.DeletePermission(c, permissionID, deleterID); err != nil {
		if err == echo_errors.ErrPermissionNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Permission not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete permission", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPermission endpoint
func (pc *PermissionController) GetPermission(c *gin.Context) {
	permissionID := c.Param("id")

	permission, err := pc.permissionService.GetPermission(c, permissionID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrPermissionNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Permission not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve permission", err)
		}
		return
	}

	c.JSON(http.StatusOK, permission)
}

// ListPermissions endpoint
func (pc *PermissionController) ListPermissions(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	permissions, err := pc.permissionService.ListPermissions(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list permissions", err)
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// SearchPermissions endpoint
func (pc *PermissionController) SearchPermissions(c *gin.Context) {
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

	permissions, err := pc.permissionService.SearchPermissions(c, query, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search permissions", err)
		return
	}

	c.JSON(http.StatusOK, permissions)
}
