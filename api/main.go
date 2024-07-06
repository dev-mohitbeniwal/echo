package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	"github.com/dev-mohitbeniwal/echo/api/config"
	"github.com/dev-mohitbeniwal/echo/api/controller"
	"github.com/dev-mohitbeniwal/echo/api/dao"
	"github.com/dev-mohitbeniwal/echo/api/db"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/middleware"
	"github.com/dev-mohitbeniwal/echo/api/service"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

func main() {
	// Initialize configuration
	if err := config.InitConfig(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// Initialize logger
	logger.InitLogger()
	defer logger.Sync()

	// Initialize Neo4j
	if err := db.InitNeo4j(); err != nil {
		logger.Fatal("Failed to initialize Neo4j", zap.Error(err))
	}
	defer db.CloseNeo4j()

	// Initialize Redis
	if err := db.InitRedis(); err != nil {
		logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}
	defer db.CloseRedis()

	// Initialize EventBus
	eventBus := util.NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventBus.Start(ctx)

	// Initialize services and utilities
	validationUtil := util.NewValidationUtil()
	cacheService := util.NewCacheService()
	notificationService := util.NewNotificationService()
	auditRepository, _ := audit.NewElasticsearchRepository(config.GetString("elasticsearch.url"))
	auditService := audit.NewService(auditRepository)

	// Initialize DAOs
	policyDAO := dao.NewPolicyDAO(db.Neo4jDriver, auditService)

	// Initialize services
	policyService := service.NewPolicyService(
		policyDAO,
		validationUtil,
		cacheService,
		notificationService,
		eventBus,
	)

	// Initialize controllers
	policyController := controller.NewPolicyController(policyService)

	// Set up Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.RateLimiter(100, time.Minute)) // 100 requests per minute

	// Register routes
	policyController.RegisterRoutes(router)

	// Set up the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.GetString("server.port")),
		Handler: router,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting server", zap.String("port", config.GetString("server.port")))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}
