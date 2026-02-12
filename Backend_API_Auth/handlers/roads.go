package handlers

import (
	"context"
	"net/http"
	"time"

	"traffic-prediction-api/models"
	"traffic-prediction-api/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoadsHandler struct {
	db    *gorm.DB
	cache *services.CacheService
}

func NewRoadsHandler(db *gorm.DB, cache *services.CacheService) *RoadsHandler {
	return &RoadsHandler{db: db, cache: cache}
}

func (h *RoadsHandler) GetRoads(c *gin.Context) {
	const cacheKey = "roads:all"

	var cached struct {
		Data []models.Road `json:"data"`
	}
	if err := h.cache.Get(c.Request.Context(), cacheKey, &cached); err == nil && cached.Data != nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	var roads []models.Road
	if err := h.db.Order("road_id").Find(&roads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database query failed"})
		return
	}

	resp := gin.H{"data": roads}
	go h.cache.Set(context.Background(), cacheKey, resp, 60*time.Second)

	c.JSON(http.StatusOK, resp)
}
