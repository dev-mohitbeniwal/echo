// api/controller/controllers.go
package controller

import "github.com/dev-mohitbeniwal/echo/api/service"

type Controllers struct {
	Policy *PolicyController
	User   *UserController
	Org    *OrganizationController
	Dept   *DepartmentController
}

func InitializeControllers(services *service.Services) *Controllers {
	return &Controllers{
		Policy: NewPolicyController(services.Policy),
		User:   NewUserController(services.User),
		Org:    NewOrganizationController(services.Org),
		Dept:   NewDepartmentController(services.Dept),
	}
}
