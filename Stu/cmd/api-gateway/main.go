package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"stu/internal/admin"
	"stu/internal/adminauth"
	"stu/internal/app"
	"stu/internal/auth"
	"stu/internal/config"
	"stu/internal/dialogs"
	"stu/internal/mailer"
	"stu/internal/middleware"
	"stu/internal/observability"
	"stu/internal/platform/postgres"
	rediscfg "stu/internal/platform/redis"
	"stu/internal/realtime"
	"stu/internal/reports"
)

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func main() {
	cfg, err := config.LoadService("API_GATEWAY_")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	logger := observability.NewLogger("api-gateway", cfg.LogLevel, cfg.Environment == "dev")
	server := app.NewHTTPServer("api-gateway", cfg, logger)

	db, err := postgres.Connect(context.Background(), cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("db connection failed")
	}
	rdb := rediscfg.New(cfg.Redis)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal().Err(err).Msg("redis connection failed")
	}
	authRepo := auth.NewRepository(db)
	validator := auth.NewAccessValidator(authRepo)
	mail := mailer.New(cfg.Mailer)
	authSvc := auth.NewService(authRepo, mail, auth.Config{
		AccessTokenTTL:      time.Minute * 15,
		RefreshTokenTTL:     time.Hour * 24 * 30,
		VerificationCodeTTL: time.Minute * 15,
	})
	dialogRepo := dialogs.NewRepository(db)
	dialogService := dialogs.NewService(dialogRepo, authRepo.GetUserByEmail)
	dialogPublisher := realtime.NewRedisPublisher(rdb)
	dialogService.SetPublisher(dialogPublisher)
	wsProxy := realtime.NewGatewayWSProxy("http://realtime:8082/v1/ws", validator, logger)
	reportsRepo := reports.NewRepository(db)
	aiClient := reports.NewAgentClient(cfg.ModerationAgentURL, logger)
	reportsService := reports.NewService(reportsRepo, aiClient, logger)
	adminUsers := admin.NewUsersRepo(db)
	adminAuthRepo := adminauth.NewRepository(db)
	adminAuthSvc := adminauth.NewService(authRepo, adminAuthRepo, authSvc, mail, logger)
	authTarget, _ := url.Parse("http://auth:8081")
	authProxy := httputil.NewSingleHostReverseProxy(authTarget)
	authProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		logger.Warn().Err(err).Msg("auth proxy error")
		rw.WriteHeader(http.StatusBadGateway)
	}

	server.Router.Route("/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})
		r.Route("/admin/auth", func(ar chi.Router) {
			ar.Use(middleware.RateLimiter(rdb, 30))
			adminauth.RegisterRoutes(ar, adminAuthSvc, logger)
		})
		r.Group(func(pr chi.Router) {
			pr.Use(auth.AuthMiddleware(logger, validator))
			pr.Get("/me", func(w http.ResponseWriter, r *http.Request) {
				uid, did, _ := auth.UserFromContext(r.Context())
				userUUID, err := uuid.Parse(uid)
				if err != nil {
					http.Error(w, "invalid user", http.StatusUnauthorized)
					return
				}
				user, err := authRepo.GetUserByID(r.Context(), userUUID)
				if err != nil {
					http.Error(w, "invalid user", http.StatusUnauthorized)
					return
				}
				writeJSON(w, map[string]any{
					"user_id":    uid,
					"device_id":  did,
					"email":      user.Email,
					"is_admin":   user.IsAdmin,
					"banned_at":  user.BannedAt,
					"ban_reason": user.BanReason,
				}, http.StatusOK)
			})
		})
		r.Route("/dialogs", func(dr chi.Router) {
			dr.Use(auth.AuthMiddleware(logger, validator))
			dialogs.RegisterHandlers(dr, dialogService, logger)
		})
		r.Get("/ws", wsProxy.Handle)
		r.Route("/reports", func(rr chi.Router) {
			rr.Use(middleware.RateLimiter(rdb, cfg.RateLimit.RequestsPerMinute))
			rr.Use(auth.AuthMiddleware(logger, validator))
			reports.RegisterUserRoutes(rr, reportsService, logger)
		})
		r.Route("/admin", func(ar chi.Router) {
			ar.Use(auth.AuthMiddleware(logger, validator))
			ar.Use(middleware.RequireAdmin)
			reports.RegisterAdminRoutes(ar, reportsService, logger)
			admin.RegisterRoutes(ar, reportsService, adminUsers, logger)
		})
	})
	// duplicate mount at root to avoid prefix issues
	server.Router.Handle("/v1/auth", authProxy)
	server.Router.Handle("/v1/auth/*", authProxy)

	adminFS := http.StripPrefix("/admin", http.FileServer(http.Dir("client/admin/public")))
	server.Router.Handle("/admin/*", adminFS)
	fileServer := http.FileServer(http.Dir("client/web/public"))
	server.Router.Handle("/*", fileServer)

	if err := server.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("api-gateway terminated")
	}
}
