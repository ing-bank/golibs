package logging

import (
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	_ = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
)

type RequestIdFormatter struct {
}

func (f *RequestIdFormatter) Format(entry *log.Entry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("empty entry")
	}

	var requestID string = "unknown"
	if entry.Context != nil {
		// TODO: the context key should be configurable
		if v := entry.Context.Value("rid"); v != nil {
			if s, ok := v.(string); ok && s != "" {
				requestID = s
			}
		}
	}

	raw, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log data: %w", err)
	}
	if len(raw) == 2 { // empty map: {}
		raw = []byte{}
	}

	const logSize = 5 * KiB // first 5KiB only
	var msg []byte
	if len(raw) > logSize {
		msg = append(raw[:logSize], []byte("...")...)
	} else {
		msg = raw
	}

	logLine := fmt.Sprintf("%v [%s] [rid:%s] %s %s\n",
		entry.Time.Format("2006/01/02 15:04:05"),
		strings.ToUpper(entry.Level.String()),
		requestID,
		entry.Message,
		msg,
	)
	return []byte(logLine), nil
}
