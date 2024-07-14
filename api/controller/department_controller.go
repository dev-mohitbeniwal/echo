// api/controller/department_controller.go
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

type DepartmentController struct {
	departmentService service.IDepartmentService
}

func NewDepartmentController(departmentService service.IDepartmentService) *DepartmentController {
	return &DepartmentController{
		departmentService: departmentService,
	}
}

// RegisterRoutes registers the API routes
func (dc *DepartmentController) RegisterRoutes(r *gin.Engine) {
	departments := r.Group("/departments")
	{
		departments.POST("", dc.CreateDepartment)
		departments.PUT("/:id", dc.UpdateDepartment)
		departments.DELETE("/:id", dc.DeleteDepartment)
		departments.GET("/:id", dc.GetDepartment)
		departments.GET("", dc.ListDepartments)
		departments.GET("/search", dc.SearchDepartments)
		departments.GET("/organization/:orgId", dc.GetDepartmentsByOrganization)
		departments.GET("/:id/hierarchy", dc.GetDepartmentHierarchy)
		departments.GET("/:id/children", dc.GetChildDepartments)
		departments.POST("/:id/move", dc.MoveDepartment)
	}
}

// CreateDepartment endpoint
func (dc *DepartmentController) CreateDepartment(c *gin.Context) {
	var dept model.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid department data", echo_errors.ErrInvalidDepartmentData)
		return
	}
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", echo_errors.ErrUnauthorized)
		return
	}

	createdDept, err := dc.departmentService.CreateDepartment(c, dept, userID)
	if err != nil {
		switch err {
		case echo_errors.ErrDepartmentConflict:
			util.RespondWithError(c, http.StatusConflict, "Department already exists", err)
		case echo_errors.ErrDatabaseOperation:
			util.RespondWithError(c, http.StatusInternalServerError, "Database operation failed", err)
		case echo_errors.ErrInternalServer:
			util.RespondWithError(c, http.StatusInternalServerError, "Internal server error", err)
		default:
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to create department", echo_errors.ErrInternalServer)
		}
		return
	}

	c.JSON(http.StatusCreated, createdDept)
}

// UpdateDepartment endpoint
func (dc *DepartmentController) UpdateDepartment(c *gin.Context) {
	deptID := c.Param("id")
	var dept model.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid department data", err)
		return
	}
	dept.ID = deptID
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	updatedDept, err := dc.departmentService.UpdateDepartment(c, dept, userID)
	if err != nil {
		if err == echo_errors.ErrDepartmentNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to update department", err)
		}
		return
	}

	c.JSON(http.StatusOK, updatedDept)
}

// DeleteDepartment endpoint
func (dc *DepartmentController) DeleteDepartment(c *gin.Context) {
	deptID := c.Param("id")
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err := dc.departmentService.DeleteDepartment(c, deptID, userID); err != nil {
		if err == echo_errors.ErrDepartmentNotFound {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to delete department", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetDepartment endpoint
func (dc *DepartmentController) GetDepartment(c *gin.Context) {
	deptID := c.Param("id")

	dept, err := dc.departmentService.GetDepartment(c, deptID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrDepartmentNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve department", err)
		}
		return
	}

	c.JSON(http.StatusOK, dept)
}

// ListDepartments endpoint
func (dc *DepartmentController) ListDepartments(c *gin.Context) {
	limit, offset, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	depts, err := dc.departmentService.ListDepartments(c, limit, offset)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to list departments", err)
		return
	}

	c.JSON(http.StatusOK, depts)
}

// SearchDepartments endpoint
func (dc *DepartmentController) SearchDepartments(c *gin.Context) {
	namePattern := c.Query("name")
	limit, _, err := helper_util.GetPaginationParams(c)
	if err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid pagination parameters", err)
		return
	}

	if namePattern == "" {
		util.RespondWithError(c, http.StatusBadRequest, "Name pattern is required", nil)
		return
	}

	depts, err := dc.departmentService.SearchDepartments(c, namePattern, limit)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to search departments", err)
		return
	}

	c.JSON(http.StatusOK, depts)
}

// GetDepartmentsByOrganization endpoint
func (dc *DepartmentController) GetDepartmentsByOrganization(c *gin.Context) {
	orgID := c.Param("orgId")

	depts, err := dc.departmentService.GetDepartmentsByOrganization(c, orgID)
	if err != nil {
		util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve departments", err)
		return
	}

	c.JSON(http.StatusOK, depts)
}

// GetDepartmentHierarchy endpoint
func (dc *DepartmentController) GetDepartmentHierarchy(c *gin.Context) {
	deptID := c.Param("id")

	hierarchy, err := dc.departmentService.GetDepartmentHierarchy(c, deptID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrDepartmentNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve department hierarchy", err)
		}
		return
	}

	c.JSON(http.StatusOK, hierarchy)
}

// GetChildDepartments endpoint
func (dc *DepartmentController) GetChildDepartments(c *gin.Context) {
	deptID := c.Param("id")

	children, err := dc.departmentService.GetChildDepartments(c, deptID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrDepartmentNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve child departments", err)
		}
		return
	}

	c.JSON(http.StatusOK, children)
}

// MoveDepartment endpoint
func (dc *DepartmentController) MoveDepartment(c *gin.Context) {
	deptID := c.Param("id")
	var moveRequest struct {
		NewParentID string `json:"newParentId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&moveRequest); err != nil {
		util.RespondWithError(c, http.StatusBadRequest, "Invalid request data", err)
		return
	}
	userID, err := util.GetUserIDFromContext(c)
	if err != nil {
		util.RespondWithError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	err = dc.departmentService.MoveDepartment(c, deptID, moveRequest.NewParentID, userID)
	if err != nil {
		if errors.Is(err, echo_errors.ErrDepartmentNotFound) {
			util.RespondWithError(c, http.StatusNotFound, "Department not found", err)
		} else {
			util.RespondWithError(c, http.StatusInternalServerError, "Failed to move department", err)
		}
		return
	}

	c.Status(http.StatusOK)
}
