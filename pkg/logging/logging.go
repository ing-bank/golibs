// Package logging provides utilities for structured logging with request tracing support.
//
// It integrates with logrus to provide trace ID aware formatting, allowing logs from the same
// request to be easily correlated and traced throughout the application.
//
// Core Features:
//
//   - Request ID Extraction: Automatically extracts request IDs from log entry contexts for
//     correlation of logs across service boundaries. Note that this is not the trace ID but a
//     unique request ID that should be set by request ID middleware.
//   - RequestIdFormatter: A custom logrus formatter that includes request IDs in log output.
//   - Log Data Truncation: Automatically truncates large log payloads (>5 KiB) to prevent log
//     bloat from large structured data.
//   - JSON Marshaling: Converts structured log fields to JSON for easy parsing and indexing.
//
// Log Format:
//
// The RequestIdFormatter produces logs in the following format:
//
//	2006/01/02 15:04:05 [LEVEL] [rid:request-id] message {json-data}
//
// Example output:
//
//	2024/05/01 10:30:45 [INFO] [rid:abc123def456] User created successfully {"user_id":"12345","email":"user@example.com"}
//	2024/05/01 10:30:46 [ERROR] [rid:abc123def456] Failed to send email {"error":"timeout","retries":3}
//
// Usage:
//
//	import (
//		"github.com/sirupsen/logrus"
//		"github.com/ing-bank/golibs/pkg/logging"
//	)
//
//	// Set the custom formatter
//	logger.SetFormatter(&logging.Config{})
//
//	// Log with request ID in context
//	ctx := context.WithValue(context.Background(), "rid", "request-123")
//	entry := logger.WithContext(ctx)
//	entry.WithFields(logrus.Fields{
//		"user_id": "12345",
//		"action":  "login",
//	}).Info("User logged in")
//
// Request ID Context:
// Request IDs are extracted from the log entry's context using the "rid" key (request ID).
// If no request ID is found in the context, "unknown" is used as the default.
//
// Log Truncation:
// Large structured data is automatically truncated to 5 KiB to prevent excessive log output.
// Truncated logs are marked with "..." at the end.
package logging

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const (
	DefaultLogLevel  = "info"
	DefaultLogFormat = "text"

	FlagLogLevel  = "log-level"
	FlagLogFormat = "log-format"
)

type Config struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

func (c *Config) Validate() error {
	if c.Level == "" {
		c.Level = DefaultLogLevel
	}
	if c.Format == "" {
		c.Format = DefaultLogFormat
	}
	_, err := log.ParseLevel(c.Level)
	if err != nil {
		return err
	}
	return err
}

func LogLevel(level string) log.Level {
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Panic(err)
	}
	return logLevel
}

func LogFormatter(format string) log.Formatter {
	switch format {
	case "request-id":
		return new(RequestIdFormatter)
	case "json":
		return &log.JSONFormatter{}
	case "text":
		fallthrough
	default:
		return &log.TextFormatter{
			FullTimestamp: true,
		}
	}
}

func SetLogFormatter(cfg *Config) {
	log.SetFormatter(LogFormatter(cfg.Format))
	log.SetLevel(LogLevel(cfg.Level))
}

func DefaultLogger() {
	SetLogFormatter(DefaultConfig())
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func (c *Config) ApplyDefaults() {
	if c.Level == "" {
		c.Level = DefaultLogLevel
	}
	if c.Format == "" {
		c.Format = DefaultLogFormat
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()
	flags.String(FlagLogLevel, c.Level, "Log level (debug, info, warn, error, fatal, panic)")
	flags.String(FlagLogFormat, c.Format, "Log format (text, json, request-id)")
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if fs.Changed(FlagLogLevel) {
		if c.Level, err = fs.GetString(FlagLogLevel); err != nil {
			return err
		}
	}
	if fs.Changed(FlagLogFormat) {
		if c.Format, err = fs.GetString(FlagLogFormat); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	if os.Getenv("PFLAGS_LOGGING_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
	DefaultLogger()
}
