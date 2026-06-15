package dynamic

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/config"
)

//func Ex() {
//	err := RegisterScopeType[basic.Scope]("basic",
//		// WithUserHeaderParser(func(c *gin.Context) ([]basic.Scope, error) { ... }),
//		// ...
//	)
//}

func RegisterScopeType[T scope.Scope](name string, option ...config.Option[*AnonScopeParser[T]]) error {
	parser := &AnonScopeParser[T]{name: name}
	if err := config.ApplyOpts(parser, option...); err != nil {
		return err
	}
	scope.RegisterParser(parser)
	return nil
}

type AnonScopeParser[T scope.Scope] struct {
	name             string
	parseUserHeaders func(c *gin.Context) []T
}

func (a AnonScopeParser[T]) GetName() string {
	return a.name
}

func (a AnonScopeParser[T]) ParseNpaScope(in json.RawMessage) (scope.Scope, error) {
	return scope.FromJSON[T](in)
}

func (a AnonScopeParser[T]) ParseCertificateScope(in json.RawMessage) (scope.Scope, error) {
	return scope.FromJSON[T](in)
}

func WithUserHeaderParser[T scope.Scope](headerParser func(c *gin.Context) []T) config.Opt[*AnonScopeParser[T]] {
	return func(f *AnonScopeParser[T]) error {
		f.parseUserHeaders = headerParser
		return nil
	}
}

func (a AnonScopeParser[T]) ParseUserHeader(c *gin.Context) []scope.Scope {
	if a.parseUserHeaders != nil {
		return scope.AsScopeSlice(a.parseUserHeaders(c))
	}
	return nil
}
