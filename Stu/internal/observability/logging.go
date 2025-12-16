package observability

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NewLogger configures zerolog with service metadata.
func NewLogger(service, level string, pretty bool) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var writer zerolog.Logger
	if pretty {
		console := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		writer = zerolog.New(console)
	} else {
		writer = zerolog.New(os.Stdout)
	}

	return writer.Level(lvl).With().Timestamp().Str("service", service).Logger()
}
