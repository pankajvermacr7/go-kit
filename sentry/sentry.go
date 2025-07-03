// sentry/sentry.go
package sentry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
)

var flushTimeout = 2 * time.Second

func Init(config SentryConfig) error {
	if !config.IsEnabled {
		log.Warn().Msg("Sentry is disabled")
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:                config.DSN,
		Environment:        config.Environment,
		Release:            config.Release,
		Debug:              config.Debug,
		TracesSampleRate:   config.TracesSampleRate,
		ProfilesSampleRate: config.ProfilesSampleRate,
		AttachStacktrace:   true,
		EnableTracing:      config.EnableTracing,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if hint != nil && hint.OriginalException != nil {
				log.Error().
					Err(hint.OriginalException).
					Str("sentry_event_id", string(event.EventID)).
					Msg("Sentry event captured")
			}
			return event
		},
	})

	if err != nil {
		return fmt.Errorf("sentry initialization failed: %w", err)
	}

	return nil
}

func CaptureError(err error) string {
	if err == nil {
		return ""
	}
	eventID := sentry.CaptureException(err)
	Flush()
	if eventID == nil {
		return ""
	}
	return string(*eventID)
}

func CaptureMessage(message string) string {
	fmt.Println("CaptureMessage: ", message)
	eventID := sentry.CaptureMessage(message)
	Flush()
	if eventID == nil {
		return ""
	}
	return string(*eventID)
}

func Flush() {
	sentry.Flush(flushTimeout)
}

func Close() {
	Flush()
	sentry.Recover()
}

func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.CurrentHub().Clone()
		ctx := sentry.SetHubOnContext(r.Context(), hub)

		defer func() {
			if err := recover(); err != nil {
				eventID := hub.RecoverWithContext(ctx, err)
				hub.Scope().SetRequest(r)
				logRequestError(hub, r, eventID, err)
				Flush()
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		hub.Scope().SetRequest(r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func logRequestError(_ *sentry.Hub, r *http.Request, eventID *sentry.EventID, err interface{}) {
	log.Error().
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("sentry_event_id", string(*eventID)).
		Interface("error", err).
		Msg("Panic recovered")
}
