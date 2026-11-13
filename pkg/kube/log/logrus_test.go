package log

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogrus(level logrus.Level) (*logrus.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := logrus.New()
	l.SetOutput(buf)
	l.SetLevel(level)
	l.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableQuote: true, DisableSorting: false})
	return l, buf
}

func TestNewLogrusLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    logrus.Level
		log      func(l *Logger)
		contains []string
		empty    bool
	}{
		{
			name:     "info with sections",
			level:    logrus.InfoLevel,
			log:      func(l *Logger) { l.Infof("hello %s", "world") },
			contains: []string{"level=info", "msg=[INFO] [ns/name] hello world"},
		},
		{
			name:     "errorf logs at error level with error field",
			level:    logrus.InfoLevel,
			log:      func(l *Logger) { l.Errorf(errors.New("boom"), "failed %d", 1) },
			contains: []string{"level=error", "msg=[ERROR] [ns/name] failed 1", "error=boom"},
		},
		{
			name:     "debugf formats args and maps to debug",
			level:    logrus.DebugLevel,
			log:      func(l *Logger) { l.Debugf("n=%d", 3) },
			contains: []string{"level=debug", "msg=[DEBUG] [ns/name] n=3"},
		},
		{
			name:  "debugf suppressed at info level",
			level: logrus.InfoLevel,
			log:   func(l *Logger) { l.Debugf("hidden") },
			empty: true,
		},
		{
			name:  "names and values are carried as fields",
			level: logrus.InfoLevel,
			log: func(l *Logger) {
				l.Logger.WithName("ctrl").WithName("widget").WithValues("a", 1).Info("msg", "b", 2, "dangling")
			},
			contains: []string{"logger=ctrl/widget", "a=1", "b=2", "dangling=(MISSING)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lr, buf := newTestLogrus(tt.level)
			l := &Logger{Logger: NewLogrusLogger(lr), Sections: []string{"ns/name"}}

			tt.log(l)

			if tt.empty {
				assert.Empty(t, buf.String())
				return
			}
			for _, s := range tt.contains {
				assert.Contains(t, buf.String(), s)
			}
		})
	}
}

func TestNewLogrusLogger_WithValuesDoesNotLeakToParent(t *testing.T) {
	lr, buf := newTestLogrus(logrus.InfoLevel)
	parent := NewLogrusLogger(lr)
	_ = parent.WithValues("child", true)

	parent.Info("parent")
	assert.NotContains(t, buf.String(), "child")
}

func TestNewLogrusLogger_DefaultsToStandardLogger(t *testing.T) {
	sink, ok := NewLogrusLogger(nil).GetSink().(*logrusSink)
	require.True(t, ok)
	assert.Same(t, logrus.StandardLogger(), sink.logger)
}
