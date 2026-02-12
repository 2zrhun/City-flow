package main

import (
	"fmt"
	"log"
	"time"

	"traffic-prediction-api/config"
	"traffic-prediction-api/handlers"
	"traffic-prediction-api/middleware"
	"traffic-prediction-api/models"
	"traffic-prediction-api/services"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.GetDSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get sql db handle: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("Failed to migrate users table: %v", err)
	}

	cache, err := services.NewCacheService(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer cache.Close()

	authService := services.NewAuthService(cfg.JWT)

	authHandler := handlers.NewAuthHandler(db, authService)
	trafficHandler := handlers.NewTrafficHandler(db, cache)
	predictionHandler := handlers.NewPredictionHandler(db, cache)
	rerouteHandler := handlers.NewRerouteHandler(db, cache)
	roadsHandler := handlers.NewRoadsHandler(db, cache)

	router := gin.Default()

	router.Use(middleware.SetupCORS(cfg.CORS))

	router.GET("/health", handlers.Health)

	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	api := router.Group("/api")
	api.Use(middleware.JWTAuth(authService))
	{
		api.POST("/auth/logout", authHandler.Logout)
		api.GET("/roads", roadsHandler.GetRoads)
		api.GET("/traffic/live", trafficHandler.GetLive)
		api.GET("/predictions", predictionHandler.GetPredictions)
		api.GET("/reroutes/recommended", rerouteHandler.GetRecommended)
	}

	router.GET("/ws/live", handlers.LiveWebSocket(cache, authService))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
