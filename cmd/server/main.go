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

	if err := db.AutoMigrate(&model.User{}, &model.ShortLink{}, &model.AccessLog{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	redisCache := cache.NewRedisCache(cfg.Redis)
	if err := redisCache.Ping(context.Background()); err != nil {
		log.Fatalf("connect redis: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	linkRepo := repository.NewLinkRepo(db)
	accessLogRepo := repository.NewAccessLogRepo(db)

	authSvc := service.NewAuthService(userRepo, cfg)
	linkSvc := service.NewLinkService(linkRepo, redisCache, cfg)
	linkSvc.StartLogWorker(accessLogRepo, 4, "access_log_fallback.tsv")

	authHandler := handler.NewAuthHandler(authSvc)
	linkHandler := handler.NewLinkHandler(linkSvc)
	pageHandler := handler.NewPageHandler(cfg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "./web/static")

	r.GET("/login", pageHandler.LoginPage)
	r.GET("/register", pageHandler.RegisterPage)

	r.GET("/s/:code", linkHandler.Redirect)

	api := r.Group("/api/v1")
	api.Use(middleware.RateLimitMiddleware(linkSvc.CheckRateLimit))
	{
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)
	}

	authed := api.Group("")
	authed.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
	{
		authed.POST("/links", linkHandler.Create)
		authed.POST("/links/batch", linkHandler.BatchCreate)
		authed.GET("/links", linkHandler.List)
		authed.GET("/links/:id", linkHandler.Get)
		authed.PUT("/links/:id", linkHandler.Update)
		authed.DELETE("/links/:id", linkHandler.Delete)
	}

	dashGroup := r.Group("/dashboard")
	dashGroup.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
	{
		dashGroup.GET("", pageHandler.DashboardPage)
	}

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

	// Flush pending access logs before exit
	linkSvc.Shutdown(5 * time.Second)
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
