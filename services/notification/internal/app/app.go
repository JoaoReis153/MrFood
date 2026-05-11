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

func newRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
	})
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
	svc := service.New(newRedisClient(cfg), newSMTPConfig(cfg), service.RateLimitConfig{
		EmailRateLimit: cfg.RateLimit.EmailRateLimit,
		RateLimitTTL:   cfg.RateLimit.TTL,
	})
	return &App{NotificationService: svc}, nil
}

func (app *App) Close(ctx context.Context) error {
	return app.NotificationService.Close()
}
