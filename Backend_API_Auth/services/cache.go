package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"traffic-prediction-api/config"

	"github.com/redis/go-redis/v9"
)

type CacheService struct {
	client *redis.Client
}

func NewCacheService(cfg config.RedisConfig) (*CacheService, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Retry up to 10 times (covers Istio sidecar startup delay)
	var lastErr error
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lastErr = client.Ping(ctx).Err()
		cancel()
		if lastErr == nil {
			return &CacheService{client: client}, nil
		}
		log.Printf("Redis ping attempt %d/10 failed: %v", i+1, lastErr)
		time.Sleep(2 * time.Second)
	}

	return &CacheService{client: nil}, fmt.Errorf("redis ping failed after 10 attempts: %w", lastErr)
}

func (s *CacheService) Client() *redis.Client {
	return s.client
}

func (s *CacheService) Available() bool {
	return s.client != nil
}

func (s *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
	if s.client == nil {
		return redis.Nil
	}
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (s *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if s.client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *CacheService) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return nil
	}
	return s.client.Del(ctx, key).Err()
}

func (s *CacheService) Publish(ctx context.Context, channel string, message interface{}) error {
	if s.client == nil {
		return nil
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return s.client.Publish(ctx, channel, data).Err()
}

func (s *CacheService) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	if s.client == nil {
		return nil
	}
	return s.client.Subscribe(ctx, channel)
}

func (s *CacheService) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}
