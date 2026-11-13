package log

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

// NewLogrusLogger returns a logr.Logger that writes to the given logrus logger,
// or to logrus.StandardLogger() when l is nil. It lets controller-runtime and
// this package share the logrus backend used by the rest of golibs (e.g.
// pkg/task/workflow), so all operator output ends up in one stream with one
// format. See ADR-0006.
//
// logr verbosity is mapped to logrus levels: V(0) is Info and every higher
// verbosity is Debug. Logger names are joined with "/" and emitted as the
// "logger" field.
func NewLogrusLogger(l *logrus.Logger) logr.Logger {
	if l == nil {
		l = logrus.StandardLogger()
	}
	return logr.New(&logrusSink{logger: l, fields: logrus.Fields{}})
}

// logrusSink implements logr.LogSink on top of logrus. It is immutable: every
// WithName/WithValues call returns a copy so derived loggers never leak fields
// into their parents.
type logrusSink struct {
	logger *logrus.Logger
	name   string
	fields logrus.Fields
}

var _ logr.LogSink = (*logrusSink)(nil)

func (s *logrusSink) Init(logr.RuntimeInfo) {}

func (s *logrusSink) Enabled(level int) bool {
	return s.logger.IsLevelEnabled(logrusLevel(level))
}

func (s *logrusSink) Info(level int, msg string, keysAndValues ...any) {
	s.entry(keysAndValues).Log(logrusLevel(level), msg)
}

func (s *logrusSink) Error(err error, msg string, keysAndValues ...any) {
	entry := s.entry(keysAndValues)
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Error(msg)
}

func (s *logrusSink) WithValues(keysAndValues ...any) logr.LogSink {
	out := s.clone()
	addPairs(out.fields, keysAndValues)
	return out
}

func (s *logrusSink) WithName(name string) logr.LogSink {
	out := s.clone()
	if out.name == "" {
		out.name = name
	} else {
		out.name = strings.Join([]string{out.name, name}, "/")
	}
	return out
}

func (s *logrusSink) clone() *logrusSink {
	fields := make(logrus.Fields, len(s.fields))
	for k, v := range s.fields {
		fields[k] = v
	}
	return &logrusSink{logger: s.logger, name: s.name, fields: fields}
}

func (s *logrusSink) entry(keysAndValues []any) *logrus.Entry {
	fields := make(logrus.Fields, len(s.fields)+len(keysAndValues)/2+1)
	for k, v := range s.fields {
		fields[k] = v
	}
	addPairs(fields, keysAndValues)
	if s.name != "" {
		fields["logger"] = s.name
	}
	return s.logger.WithFields(fields)
}

// addPairs copies logr key/value pairs into fields. A dangling key is kept with
// a placeholder value instead of being dropped, so malformed calls stay visible.
func addPairs(fields logrus.Fields, keysAndValues []any) {
	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprint(keysAndValues[i])
		if i+1 < len(keysAndValues) {
			fields[key] = keysAndValues[i+1]
		} else {
			fields[key] = "(MISSING)"
		}
	}
}

func logrusLevel(v int) logrus.Level {
	if v <= 0 {
		return logrus.InfoLevel
	}
	return logrus.DebugLevel
}
