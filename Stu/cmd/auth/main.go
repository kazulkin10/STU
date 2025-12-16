package main

import (
	"context"
	"log"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"stu/internal/app"
	"stu/internal/auth"
	"stu/internal/config"
	"stu/internal/mailer"
	"stu/internal/middleware"
	"stu/internal/observability"
	"stu/internal/platform/postgres"
	rediscfg "stu/internal/platform/redis"
)

func main() {
	cfg, err := config.LoadService("AUTH_")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	logger := observability.NewLogger("auth", cfg.LogLevel, cfg.Environment == "dev")
	server := app.NewHTTPServer("auth", cfg, logger)
	logger.Info().Str("database_url", cfg.Database.URL).Msg("config loaded")

	ctx := context.Background()
	db, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("db connection failed")
	}
	rdb := rediscfg.New(cfg.Redis)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("redis connection failed")
	}

	authRepo := auth.NewRepository(db)
	authMailer := mailer.New(cfg.Mailer)
	authService := auth.NewService(authRepo, authMailer, auth.Config{
		AccessTokenTTL:      time.Minute * 15,
		RefreshTokenTTL:     time.Hour * 24 * 30,
		VerificationCodeTTL: time.Minute * 15,
	})

	server.Router.Route("/v1", func(r chi.Router) {
		r.Use(middleware.RateLimiter(rdb, cfg.RateLimit.RequestsPerMinute))
		auth.RegisterHandlers(r, authService, logger)
	})

	if err := server.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("auth terminated")
	}

	// Ensure Redis is closed on shutdown
	defer func(rdb *redis.Client) {
		_ = rdb.Close()
	}(rdb)
}
