package ginserver

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWithMode(t *testing.T) {
	e := &Engine{}
	opt := WithMode(ModeRelease)
	err := opt(e)
	assert.NoError(t, err)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())
}

func TestWithHealthChecks_Success(t *testing.T) {
	e := DefaultEngine()
	opt := WithHealthChecks()
	err := opt(e)
	assert.NoError(t, err)
	assert.NotNil(t, e.Services)
	assert.NotNil(t, e.Services.prober)
}
