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

	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	"github.com/dev-mohitbeniwal/echo/api/config"
	"github.com/dev-mohitbeniwal/echo/api/controller"
	"github.com/dev-mohitbeniwal/echo/api/db"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	router "github.com/dev-mohitbeniwal/echo/api/router"
	"github.com/dev-mohitbeniwal/echo/api/service"
	"github.com/dev-mohitbeniwal/echo/api/util"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	// Initialize configuration
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Initialize logger
	logger.InitLogger(config.GetString("log.file"))
	defer logger.Sync()

	// Initialize Neo4j
	if err := db.InitNeo4j(); err != nil {
		return fmt.Errorf("failed to initialize Neo4j: %w", err)
	}
	defer db.CloseNeo4j()

	// Initialize Redis
	if err := db.InitRedis(); err != nil {
		return fmt.Errorf("failed to initialize Redis: %w", err)
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
	auditRepository, err := audit.NewElasticsearchRepository(config.GetString("elasticsearch.url"))
	if err != nil {
		return fmt.Errorf("failed to create audit repository: %w", err)
	}
	auditService := audit.NewService(auditRepository)

	services, err := service.InitializeServices(db.Neo4jDriver, auditService, validationUtil, cacheService, notificationService, eventBus)
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	controllers := controller.InitializeControllers(services)

	rateLimitRequests := config.GetInt("rate_limit.requests")
	rateLimitDuration := config.GetDuration("rate_limit.duration")
	router := router.SetupRouter(controllers, rateLimitRequests, rateLimitDuration)

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
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logger.Info("Server exiting")
	return nil
}
