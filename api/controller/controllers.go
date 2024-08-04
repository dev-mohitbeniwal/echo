// api/controller/controllers.go
package controller

import "github.com/dev-mohitbeniwal/echo/api/service"

type Controllers struct {
	Policy         *PolicyController
	User           *UserController
	Org            *OrganizationController
	Dept           *DepartmentController
	Role           *RoleController
	Group          *GroupController
	Permission     *PermissionController
	Resource       *ResourceController
	ResourceType   *ResourceTypeController
	AttributeGroup *AttributeGroupController
}

func InitializeControllers(services *service.Services) *Controllers {
	return &Controllers{
		Policy:         NewPolicyController(services.Policy),
		User:           NewUserController(services.User),
		Org:            NewOrganizationController(services.Org),
		Dept:           NewDepartmentController(services.Dept),
		Role:           NewRoleController(services.Role),
		Group:          NewGroupController(services.Group),
		Permission:     NewPermissionController(services.Permission),
		Resource:       NewResourceController(services.Resource),
		ResourceType:   NewResourceTypeController(services.ResourceTypeService),
		AttributeGroup: NewAttributeGroupController(services.AttributeGroupService),
	}
}
