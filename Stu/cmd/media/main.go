package main

import (
	"context"
	"log"

	"stu/internal/app"
	"stu/internal/config"
	"stu/internal/observability"
)

func main() {
	cfg, err := config.LoadService("MEDIA_")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	logger := observability.NewLogger("media", cfg.LogLevel, cfg.Environment == "dev")
	server := app.NewHTTPServer("media", cfg, logger)

	if err := server.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("media terminated")
	}
}
