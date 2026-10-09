package dynamic

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

func TestRegisterScopeType(t *testing.T) {
	err := RegisterScopeType[basic.Scope]("dynamic",
		WithUserHeaderParser(func(c *gin.Context, header string) []basic.Scope {
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("failed to register scope type: %v", err)
	}
}
