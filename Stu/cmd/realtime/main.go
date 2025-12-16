package main

import (
	"context"
	"log"

	"github.com/go-chi/chi/v5"

	"stu/internal/app"
	"stu/internal/auth"
	"stu/internal/config"
	"stu/internal/observability"
	"stu/internal/platform/postgres"
	rediscfg "stu/internal/platform/redis"
	"stu/internal/realtime"
)

func main() {
	cfg, err := config.LoadService("REALTIME_")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	logger := observability.NewLogger("realtime", cfg.LogLevel, cfg.Environment == "dev")
	server := app.NewHTTPServer("realtime", cfg, logger)

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
	validator := auth.NewAccessValidator(authRepo)
	hub := realtime.NewHub(logger, rdb, validator)

	server.Router.Route("/v1", func(r chi.Router) {
		r.Get("/ws", hub.HandleWS)
	})

	if err := server.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("realtime terminated")
	}
}
