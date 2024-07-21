// api/service/services.go
package service

import (
	"github.com/dev-mohitbeniwal/echo/api/audit"
	"github.com/dev-mohitbeniwal/echo/api/dao"
	"github.com/dev-mohitbeniwal/echo/api/util"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Services struct {
	Policy IPolicyService
	User   IUserService
	Org    IOrganizationService
	Dept   IDepartmentService
}

func InitializeServices(
	driver neo4j.Driver,
	auditService audit.Service,
	validationUtil *util.ValidationUtil,
	cacheService *util.CacheService,
	notificationSvc *util.NotificationService,
	eventBus *util.EventBus,
) (*Services, error) {
	policyDAO := dao.NewPolicyDAO(driver, auditService)
	userDAO := dao.NewUserDAO(driver, auditService)
	organizationDAO := dao.NewOrganizationDAO(driver, auditService)
	departmentDAO := dao.NewDepartmentDAO(driver, auditService)

	services := &Services{
		Policy: NewPolicyService(policyDAO, validationUtil, cacheService, notificationSvc, eventBus),
		User:   NewUserService(userDAO, validationUtil, cacheService, notificationSvc, eventBus),
		Org:    NewOrganizationService(organizationDAO, validationUtil, cacheService, notificationSvc, eventBus),
		Dept:   NewDepartmentService(departmentDAO, validationUtil, cacheService, notificationSvc, eventBus),
	}

	return services, nil
}
