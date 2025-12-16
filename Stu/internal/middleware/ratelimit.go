package middleware

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter limits requests per minute per IP+path using Redis INCR with TTL.
func RateLimiter(rdb *redis.Client, requestsPerMinute int) func(http.Handler) http.Handler {
	limit := int64(requestsPerMinute)
	if limit <= 0 {
		limit = 60
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}
			key := "rl:" + r.URL.Path + ":" + ip
			pipe := rdb.TxPipeline()
			incr := pipe.Incr(r.Context(), key)
			pipe.Expire(r.Context(), key, time.Minute)
			_, _ = pipe.Exec(r.Context())
			if incr.Val() > limit {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
