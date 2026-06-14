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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/handler"
	"github.com/42ShortHub/shortlink/internal/middleware"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
	"github.com/42ShortHub/shortlink/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.ShortLink{},
		&model.AccessLog{},
		&model.APIKey{},
		&model.AuditLog{},
		&model.BatchJob{},
	); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	redisCache := cache.NewRedisCache(cfg.Redis)
	if err := redisCache.Ping(context.Background()); err != nil {
		log.Fatalf("connect redis: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepo(db)
	linkRepo := repository.NewLinkRepo(db)
	accessLogRepo := repository.NewAccessLogRepo(db)
	apiKeyRepo := repository.NewAPIKeyRepo(db)
	auditLogRepo := repository.NewAuditLogRepo(db)
	batchJobRepo := repository.NewBatchJobRepo(db)

	// Services
	authSvc := service.NewAuthService(userRepo, cfg)
	linkSvc := service.NewLinkService(linkRepo, redisCache, cfg)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, redisCache, cfg)
	analyticsSvc := service.NewAnalyticsService(accessLogRepo, linkRepo, redisCache, cfg)
	auditSvc := service.NewAuditService(auditLogRepo)
	batchSvc := service.NewBatchService(batchJobRepo, linkSvc, cfg)
	qrSvc := service.NewQRCodeService(cfg)

	linkSvc.SetAnalyticsService(analyticsSvc)
	linkSvc.StartLogWorker(accessLogRepo, 4, "access_log_fallback.tsv")
	auditSvc.Start(2)
	batchSvc.Start(cfg.App.BatchWorkers)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	linkHandler := handler.NewLinkHandler(linkSvc, qrSvc)
	pageHandler := handler.NewPageHandler(cfg)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeySvc)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsSvc, linkSvc)
	batchHandler := handler.NewBatchHandler(batchSvc)
	adminHandler := handler.NewAdminHandler(userRepo, linkRepo, accessLogRepo, auditSvc, analyticsSvc)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "./web/static")

	// Public routes
	r.GET("/login", pageHandler.LoginPage)
	r.GET("/register", pageHandler.RegisterPage)
	r.GET("/s/:code", linkHandler.Redirect)

	// API v1
	api := r.Group("/api/v1")
	api.Use(middleware.RateLimitMiddleware(linkSvc.CheckRateLimit))
	{
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)
	}

	// Authenticated API (supports both JWT and API Key)
	authed := api.Group("")
	authed.Use(middleware.CombinedAuthMiddleware(cfg.JWT.Secret, apiKeySvc))
	{
		// Links CRUD
		authed.POST("/links", auditWrap(auditSvc, model.AuditCreateLink, "link", linkHandler.Create))
		authed.POST("/links/batch", linkHandler.BatchCreate)
		authed.GET("/links", linkHandler.List)
		authed.GET("/links/:id", linkHandler.Get)
		authed.PUT("/links/:id", auditWrap(auditSvc, model.AuditUpdateLink, "link", linkHandler.Update))
		authed.DELETE("/links/:id", auditWrap(auditSvc, model.AuditDeleteLink, "link", linkHandler.Delete))
		authed.GET("/links/:id/qrcode", linkHandler.QRCode)

		// Analytics
		authed.GET("/links/:id/analytics", analyticsHandler.Summary)
		authed.GET("/links/:id/analytics/timeseries", analyticsHandler.Timeseries)
		authed.GET("/links/:id/analytics/referers", analyticsHandler.Referers)
		authed.GET("/links/:id/analytics/devices", analyticsHandler.Devices)
		authed.GET("/links/:id/analytics/geo", analyticsHandler.Geo)
		authed.GET("/links/:id/analytics/realtime", analyticsHandler.Realtime)

		// API Keys
		authed.POST("/api-keys", auditWrap(auditSvc, model.AuditCreateKey, "api_key", apiKeyHandler.Create))
		authed.GET("/api-keys", apiKeyHandler.List)
		authed.DELETE("/api-keys/:id", auditWrap(auditSvc, model.AuditRevokeKey, "api_key", apiKeyHandler.Revoke))
		authed.GET("/api-keys/:id/usage", apiKeyHandler.Usage)

		// Batch operations
		authed.POST("/links/batch/async", batchHandler.SubmitAsync)
		authed.POST("/links/batch/csv", batchHandler.UploadCSV)
		authed.GET("/batch-jobs", batchHandler.ListJobs)
		authed.GET("/batch-jobs/:id", batchHandler.GetJob)
		authed.GET("/batch-jobs/:id/results", batchHandler.GetResults)
	}

	// Admin API (requires admin role)
	admin := api.Group("/admin")
	admin.Use(middleware.CombinedAuthMiddleware(cfg.JWT.Secret, apiKeySvc))
	admin.Use(middleware.RequireAdminMiddleware())
	{
		admin.GET("/overview", adminHandler.Overview)
		admin.GET("/traffic", adminHandler.Traffic)
		admin.GET("/top-links", adminHandler.TopLinks)
		admin.GET("/audit-log", adminHandler.AuditLog)
	}

	// Dashboard pages (JWT cookie auth)
	dashGroup := r.Group("/dashboard")
	dashGroup.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
	{
		dashGroup.GET("", pageHandler.DashboardPage)
		dashGroup.GET("/links/:id/analytics", pageHandler.AnalyticsPage)
		dashGroup.GET("/admin", pageHandler.AdminPage)
		dashGroup.GET("/api-keys", pageHandler.APIKeysPage)
	}

	// Materialized view refresh goroutine
	go refreshMaterializedView(db, cfg.Analytics.MVRefreshInterval)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		fmt.Printf("Server starting on :%s\n", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}

	linkSvc.Shutdown(5 * time.Second)
	auditSvc.Shutdown()
	batchSvc.Shutdown()
	fmt.Println("Server stopped")
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		if c.Request.URL.Path != "/health" {
			log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
		}
	}
}

func auditWrap(auditSvc *service.AuditService, action, resource string, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c)
		if c.Writer.Status() < 400 {
			userID := c.GetUint("user_id")
			var apiKeyID *uint
			if id, exists := c.Get("api_key_id"); exists {
				v := id.(uint)
				apiKeyID = &v
			}
			auditSvc.Record(userID, apiKeyID, action, resource, nil, nil, c.ClientIP())
		}
	}
}

func refreshMaterializedView(db *gorm.DB, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		result := db.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_daily_clicks")
		if result.Error != nil {
			log.Printf("[mv-refresh] failed: %v", result.Error)
		}
	}
}
