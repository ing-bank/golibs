package log

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Logger struct {
	logr.Logger
	Sections []string
}

func NewLogger(sections ...string) *Logger {
	// TODO: support human / JSON logging
	return &Logger{
		Sections: sections,
		Logger:   logf.Log,
	}
}

func NewLoggerFromContext(ctx context.Context) *Logger {
	logger := ctx.Value("logger")
	if logger == nil {
		return NewLogger()
	}
	return logger.(*Logger)
}

func NewContextLogger(ctx context.Context, sections ...string) (context.Context, *Logger) {
	log := NewLogger(sections...)
	return context.WithValue(ctx, "logger", log), log
}

func (l *Logger) WithSectionContext(ctx context.Context, section string) (context.Context, *Logger) {
	log := l.WithSection(section)
	return context.WithValue(ctx, "logger", log), log
}

func (l *Logger) Format(format string, args ...any) string {
	var line string
	for _, section := range l.Sections {
		line = fmt.Sprintf("%s[%s] ", line, section)
	}

	return fmt.Sprintf(line+format, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	l.Logger.Info("[INFO] " + l.Format(format, args...))
}

func (l *Logger) Errorf(err error, format string, args ...any) {
	l.Logger.Info("[ERROR] " + fmt.Sprintf("%s: %s", l.Format(format, args...), err.Error()))
}

//func (l *LogWrapper) ErrorMsg(format string, args ...any) {
//	l.Logger.Error(fmt.Errorf(format, args...), "")
//}

func (l *Logger) Debugf(format string, args ...any) {
	l.Logger.V(4).Info("[DEBUG] "+format, args...)
}

func (l *Logger) WithSection(sections ...string) *Logger {
	return NewLogger(append(l.Sections, sections...)...)
}
