package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"traffic-prediction-api/models"
	"traffic-prediction-api/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TrafficHandler struct {
	db    *gorm.DB
	cache *services.CacheService
}

func NewTrafficHandler(db *gorm.DB, cache *services.CacheService) *TrafficHandler {
	return &TrafficHandler{db: db, cache: cache}
}

func (h *TrafficHandler) GetLive(c *gin.Context) {
	p := ParsePagination(c)
	roadID := c.Query("road_id")

	beforeStr := ""
	if p.Before != nil {
		beforeStr = p.Before.Format(time.RFC3339Nano)
	}
	cacheKey := fmt.Sprintf("traffic:live:%s:%d:%s", roadID, p.Limit, beforeStr)

	var cached CursorResponse
	if err := h.cache.Get(c.Request.Context(), cacheKey, &cached); err == nil && cached.Data != nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	query := h.db.Model(&models.TrafficRaw{}).Order("ts DESC").Limit(p.Limit + 1)
	if p.Before != nil {
		query = query.Where("ts < ?", *p.Before)
	}
	if roadID != "" {
		query = query.Where("road_id = ?", roadID)
	}

	var rows []models.TrafficRaw
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database query failed"})
		return
	}

	hasMore := len(rows) > p.Limit
	if hasMore {
		rows = rows[:p.Limit]
	}

	var nextCursor string
	if hasMore && len(rows) > 0 {
		nextCursor = rows[len(rows)-1].TS.Format(time.RFC3339Nano)
	}

	resp := CursorResponse{Data: rows, NextCursor: nextCursor, HasMore: hasMore}
	go h.cache.Set(context.Background(), cacheKey, resp, 5*time.Second)

	c.JSON(http.StatusOK, resp)
}
