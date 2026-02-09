package handlers

import (
	"context"
	"log"
	"net/http"

	"traffic-prediction-api/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func LiveWebSocket(cache *services.CacheService, authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token query parameter"})
			return
		}

		if _, err := authService.ValidateToken(tokenStr); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		// Read pump: detect client disconnect
		go func() {
			defer cancel()
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		// Subscribe to Redis pub/sub channel
		pubsub := cache.Subscribe(ctx, "cityflow:live")
		defer pubsub.Close()

		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				err := conn.WriteJSON(gin.H{
					"type": "traffic_update",
					"data": msg.Payload,
				})
				if err != nil {
					log.Printf("ws write error: %v", err)
					return
				}
			}
		}
	}
}
