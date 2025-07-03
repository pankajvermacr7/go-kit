package sentry

type SentryConfig struct {
	DSN                string  `mapstructure:"dsn"`
	Environment        string  `mapstructure:"environment"`
	Release            string  `mapstructure:"release"`
	Debug              bool    `mapstructure:"debug"`
	IsEnabled          bool    `mapstructure:"is_enabled"`
	TracesSampleRate   float64 `mapstructure:"traces_sample_rate"`
	ProfilesSampleRate float64 `mapstructure:"profiles_sample_rate"`
	EnableTracing      bool    `mapstructure:"enable_tracing"`
}
