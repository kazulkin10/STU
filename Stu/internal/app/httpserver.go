package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"stu/internal/config"
	"stu/internal/handlers"
	"stu/internal/middleware"
)

// HTTPServer wraps base HTTP server setup.
type HTTPServer struct {
	Name   string
	Config config.ServiceConfig
	Router *chi.Mux
	Logger zerolog.Logger
}

// NewHTTPServer configures router with baseline middleware and health endpoints.
func NewHTTPServer(name string, cfg config.ServiceConfig, logger zerolog.Logger) *HTTPServer {
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(chimw.RealIP)
	router.Use(chimw.Recoverer)
	router.Use(chimw.Timeout(60 * time.Second))
	router.Use(middleware.SecurityHeaders(cfg.Security.EnableHSTS))
	router.Use(middleware.CORSMiddleware(cfg.Security.AllowedOrigins))
	router.Use(middleware.RequestLogger(logger))

	router.Get("/healthz", handlers.HealthHandler(name))
	router.Get("/ready", handlers.HealthHandler(name))

	return &HTTPServer{
		Name:   name,
		Config: cfg,
		Router: router,
		Logger: logger,
	}
}

// Start boots HTTP and metrics listeners with graceful shutdown.
func (s *HTTPServer) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.Config.HTTP.Addr,
		Handler:      s.Router,
		ReadTimeout:  s.Config.HTTP.ReadTimeout,
		WriteTimeout: s.Config.HTTP.WriteTimeout,
		IdleTimeout:  s.Config.HTTP.IdleTimeout,
	}

	metricsSrv := &http.Server{
		Addr:    s.Config.Metrics.Addr,
		Handler: promhttp.Handler(),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.Logger.Error().Err(err).Msg("metrics server failed")
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	s.Logger.Info().Str("addr", s.Config.HTTP.Addr).Msg("http server starting")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
