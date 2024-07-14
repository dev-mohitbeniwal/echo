// api/controller/user_controller.go
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

type UserController struct {
	userService service.IUserService
}

func NewUserController(userService service.IUserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

// RegisterRoutes registers the API routes
func (uc *UserController) RegisterRoutes(r *gin.Engine) {
	users := r.Group("/users")
	{
		users.POST("", uc.CreateUser)
		users.PUT("/:id", uc.UpdateUser)
		users.DELETE("/:id", uc.DeleteUser)
		users.GET("/:id", uc.GetUser)
		users.GET("", uc.ListUsers)
		users.GET("/search", uc.SearchUsers)
	}
}

// CreateUser endpoint
func (uc *UserController) CreateUser(c *gin.Context) {
	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid user data", echo_errors.ErrInvalidUserData)
		return
	}
	creatorID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdUser, err := uc.userService.CreateUser(c, user, creatorID)
	if err != nil {
		switch err {
		case echo_errors.ErrUserConflict:
			util.RespondWithError(c, http.StatusConflict, "User already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create user", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

// UpdateUser endpoint
func (uc *UserController) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid user data", err)
		return
	}
	user.ID = userID
	updaterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedUser, err := uc.userService.UpdateUser(c, user, updaterID)
	if err != nil {
		if err == echo_errors.ErrUserNotFound {
			util.RespondWithError(c, http.StatusNotFound, "User not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update user", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// DeleteUser endpoint
func (uc *UserController) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	deleterID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := uc.userService.DeleteUser(c, userID, deleterID); err != nil {
		if err == echo_errors.ErrUserNotFound {
			util.RespondWithError(c, http.StatusNotFound, "User not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete user", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetUser endpoint
func (uc *UserController) GetUser(c *gin.Context) {
	userID := c.Param("id")

	user, err := uc.userService.GetUser(c, userID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrUserNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "User not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve user", err)
		}
		return
	}

	c.JSON(http.StatusOK, user)
}

// ListUsers endpoint
func (uc *UserController) ListUsers(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	users, err := uc.userService.ListUsers(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list users", err)
		return
	}

	c.JSON(http.StatusOK, users)
}

// SearchUsers endpoint
func (uc *UserController) SearchUsers(c *gin.Context) {
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

	users, err := uc.userService.SearchUsers(c, query, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search users", err)
		return
	}

	c.JSON(http.StatusOK, users)
}
