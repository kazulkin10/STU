package config

import (
	"time"

	"github.com/caarlos0/env/v10"
)

// HTTPConfig describes HTTP server settings.
type HTTPConfig struct {
	Addr         string        `env:"HTTP_ADDR" envDefault:":8080"`
	ReadTimeout  time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout  time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
}

// GRPCConfig describes gRPC listener settings.
type GRPCConfig struct {
	Addr string `env:"GRPC_ADDR" envDefault:":9000"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL               string        `env:"DATABASE_URL" envDefault:"postgres://stu:stu@localhost:5432/stu?sslmode=disable"`
	MaxConns          int32         `env:"DB_MAX_CONNS" envDefault:"16"`
	MaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"10m"`
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"30s"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

// MinIOConfig holds object storage settings.
type MinIOConfig struct {
	Endpoint  string `env:"MINIO_ENDPOINT" envDefault:"localhost:9000"`
	AccessKey string `env:"MINIO_ACCESS_KEY" envDefault:"minioadmin"`
	SecretKey string `env:"MINIO_SECRET_KEY" envDefault:"minioadmin"`
	Bucket    string `env:"MINIO_BUCKET" envDefault:"stu-media"`
	UseSSL    bool   `env:"MINIO_USE_SSL" envDefault:"false"`
}

// MailerConfig contains e-mail delivery settings.
type MailerConfig struct {
	Mode     string `env:"MAIL_MODE" envDefault:"mailpit"` // mailpit|mailersend
	APIKey   string `env:"MAILERSEND_API_KEY"`
	From     string `env:"MAIL_FROM" envDefault:"no-reply@stu.local"`
	FromName string `env:"MAIL_FROM_NAME" envDefault:"Stu"`
	SMTPHost string `env:"SMTP_HOST" envDefault:"mailpit"`
	SMTPPort int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUser string `env:"SMTP_USER" envDefault:""`
	SMTPPass string `env:"SMTP_PASS" envDefault:""`
}

// TimewebAgentConfig stores Timeweb Cloud agent credentials.
type TimewebAgentConfig struct {
	URL      string `env:"TIMEWEB_AGENT_URL" envDefault:"https://agent.timeweb.cloud/api/v1/cloud-ai/agents/6087a6cd-b070-4bcc-8b71-9aaed01c2168/v1"`
	AccessID string `env:"TIMEWEB_AGENT_ACCESS_ID" envDefault:"6087a6cd-b070-4bcc-8b71-9aaed01c2168"`
	APIKey   string `env:"TIMEWEB_AGENT_API_KEY"`
}

// SecurityConfig sets security-related toggles.
type SecurityConfig struct {
	AllowedOrigins string `env:"CORS_ALLOWED_ORIGINS" envDefault:"*"`
	EnableHSTS     bool   `env:"ENABLE_HSTS" envDefault:"false"`
}

// MetricsConfig controls metrics/pprof listeners.
type MetricsConfig struct {
	Addr string `env:"METRICS_ADDR" envDefault:":9090"`
}

// RateLimitConfig controls request limits per minute.
type RateLimitConfig struct {
	RequestsPerMinute int `env:"RATE_LIMIT_RPM" envDefault:"60"`
}

// ServiceConfig is the shared config across services.
type ServiceConfig struct {
	ServiceName        string `env:"SERVICE_NAME" envDefault:"stu"`
	Environment        string `env:"ENV" envDefault:"dev"`
	HTTP               HTTPConfig
	GRPC               GRPCConfig
	Database           DatabaseConfig
	Redis              RedisConfig
	MinIO              MinIOConfig
	Mailer             MailerConfig
	Timeweb            TimewebAgentConfig
	ModerationAgentURL string `env:"MOD_AGENT_URL" envDefault:"http://moderation-agent:8085/analyze"`
	Security           SecurityConfig
	Metrics            MetricsConfig
	RateLimit          RateLimitConfig
	LogLevel           string `env:"LOG_LEVEL" envDefault:"info"`
}

// LoadService parses environment variables with a prefix (e.g. API_GATEWAY_).
func LoadService(prefix string) (ServiceConfig, error) {
	var cfg ServiceConfig
	// Parse without prefix first to allow shared variables, then override with prefix-specific values.
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	opts := env.Options{Prefix: prefix}
	err := env.ParseWithOptions(&cfg, opts)
	return cfg, err
}
