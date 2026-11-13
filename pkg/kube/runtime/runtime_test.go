package runtime

import (
	"testing"

	"github.com/go-logr/logr"
	kubelog "github.com/ing-bank/golibs/pkg/kube/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestConfigApplyDefaults_Logger(t *testing.T) {
	custom := kubelog.NewLogrusLogger(logrus.New())

	tests := []struct {
		name   string
		logger logr.Logger
		check  func(t *testing.T, got logr.Logger)
	}{
		{
			name: "defaults to logrus bridge",
			check: func(t *testing.T, got logr.Logger) {
				assert.NotNil(t, got.GetSink())
			},
		},
		{
			name:   "keeps caller logger",
			logger: custom,
			check: func(t *testing.T, got logr.Logger) {
				assert.Same(t, custom.GetSink(), got.GetSink())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config[*Widget]{Logger: tt.logger}
			cfg.ApplyDefaults()
			tt.check(t, cfg.Logger)
		})
	}
}
