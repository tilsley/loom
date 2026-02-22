package logger

import (
	"log/slog"

	"github.com/tilsley/loom/pkg/logging"
)

// New returns a logger configured from LOG_FORMAT and LOG_LEVEL env vars.
// See pkg/logging for details.
func New() *slog.Logger {
	return logging.New()
}
