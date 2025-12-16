package realtime

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/rs/zerolog"

	"stu/internal/auth"
)

// GatewayWSProxy proxies /v1/ws through gateway to realtime.
type GatewayWSProxy struct {
	target    *url.URL
	validator auth.AccessValidator
	logger    zerolog.Logger
}

func NewGatewayWSProxy(target string, validator auth.AccessValidator, logger zerolog.Logger) *GatewayWSProxy {
	u, _ := url.Parse(target)
	return &GatewayWSProxy{target: u, validator: validator, logger: logger}
}

func (p *GatewayWSProxy) Handle(w http.ResponseWriter, r *http.Request) {
	// accept token in header or query
	authz := r.Header.Get("Authorization")
	if authz == "" {
		if tok := r.URL.Query().Get("token"); tok != "" {
			r.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	// fast precheck
	if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(hdr, "Bearer"))
		sum := sha256.Sum256([]byte(token))
		if _, err := p.validator.ValidateAccessToken(context.Background(), sum[:]); err != nil {
			if err == auth.ErrBanned {
				http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(p.target)
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		p.logger.Warn().Err(err).Msg("ws proxy error")
		rw.WriteHeader(http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}
