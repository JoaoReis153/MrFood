package app

import (
	"MrFood/services/notification/config"
	"MrFood/services/notification/internal/service"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type App struct {
	NotificationService *service.Service
}

func newRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

func newSMTPConfig(cfg *config.Config) service.SMTPConfig {
	return service.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		User:     cfg.SMTP.User,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	redisClient, err := newRedisClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	svc := service.New(redisClient, newSMTPConfig(cfg), service.RateLimitConfig{
		EmailRateLimit: cfg.RateLimit.EmailRateLimit,
		RateLimitTTL:   cfg.RateLimit.TTL,
	})
	return &App{NotificationService: svc}, nil
}

func (app *App) Close(ctx context.Context) error {
	return app.NotificationService.Close()
}
