// logging/logging.go
package logging

import (
	"os"
	"runtime"
	"time"

	"github.com/pankajvermacr7/gokit/sentry"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// SentryHook is a custom hook for zerolog to send logs to Sentry.
type SentryHook struct{}

// Run implements the zerolog Hook interface.
func (h SentryHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// Only proceed if the level is Error or higher
	if level < zerolog.ErrorLevel {
		return
	}
	// Send to Sentry
	eventID := sentry.CaptureMessage(msg)

	// Add Sentry event ID to log
	if eventID != "" {
		e.Str("sentry_event_id", eventID)
	}
}

// InitLogger initializes the logger.
func InitLogger() {
	// Set log level based on environment
	configureLogLevel()

	// Set up zerolog with console writer
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    os.Getenv("ENVIRONMENT") == "prod",
	}

	// Configure global logger
	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	// Add Sentry hook to zerolog
	log.Logger = log.Logger.Hook(SentryHook{})
}

func configureLogLevel() {
	env := os.Getenv("ENVIRONMENT")
	if env == "prod" || env == "dev" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		return
	}

	level := viper.GetString("logging.level")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func NewLogger() zerolog.Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Stack().
		Logger().
		Hook(SentryHook{})

	return logger
}

func LogError(err error, msg string) {
	logger := NewLogger()

	// Ensure we have a stack trace
	if _, ok := err.(stackTracer); !ok {
		err = withStack(err)
	}

	// Capture in Sentry
	eventID := sentry.CaptureError(err)

	// Log with zerolog
	logger.Error().
		Err(err).
		Str("sentry_event_id", eventID).
		Msg(msg)
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func withStack(err error) error {
	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	return errors.WithStack(err)
}
