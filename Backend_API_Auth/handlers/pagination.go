package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

type PaginationParams struct {
	Limit  int
	Before *time.Time
}

type CursorResponse struct {
	Data       interface{} `json:"data"`
	NextCursor string      `json:"next_cursor,omitempty"`
	HasMore    bool        `json:"has_more"`
}

func ParsePagination(c *gin.Context) PaginationParams {
	p := PaginationParams{Limit: DefaultLimit}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			p.Limit = l
		}
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}

	if beforeStr := c.Query("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, beforeStr); err == nil {
			p.Before = &t
		}
	}

	return p
}
