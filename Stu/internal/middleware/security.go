package middleware

import (
	"net/http"
	"strings"
)

// CORSMiddleware allows configurable origins.
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	origins := map[string]struct{}{}
	for _, origin := range strings.Split(allowedOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		origins[origin] = struct{}{}
	}

	allowAll := len(origins) == 0 || (len(origins) == 1 && hasOrigin(origins, "*"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowAll || hasOrigin(origins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", originOrWildcard(origin, allowAll))
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets baseline security headers.
func SecurityHeaders(enableHSTS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			if enableHSTS {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasOrigin(origins map[string]struct{}, target string) bool {
	_, ok := origins[target]
	return ok
}

func originOrWildcard(origin string, allowAll bool) string {
	if allowAll {
		return "*"
	}
	return origin
}
