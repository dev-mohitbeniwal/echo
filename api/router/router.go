// api/router/router.go

package router

import (
	"time"

	"github.com/dev-mohitbeniwal/echo/api/controller"
	"github.com/dev-mohitbeniwal/echo/api/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter(
	controllers *controller.Controllers,
	rateLimitRequests int,
	rateLimitDuration time.Duration,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.RateLimiter(rateLimitRequests, rateLimitDuration))
	router.Use(middleware.GroupAuthMiddleware([]string{"alive-admin"}))

	api := router.Group("/api/v1")

	controllers.Policy.RegisterRoutes(api)
	controllers.User.RegisterRoutes(api)
	controllers.Org.RegisterRoutes(api)
	controllers.Dept.RegisterRoutes(api)
	controllers.Role.RegisterRoutes(api)
	controllers.Group.RegisterRoutes(api)
	controllers.Permission.RegisterRoutes(api)

	return router
}
