package access

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
)

type TrustLevel string

const (
	TrustCertificate TrustLevel = "Certificate"
	TrustNPA         TrustLevel = "NPA"
	TrustFedSVC      TrustLevel = "Azure Federated ServiceConnection"
	TrustUser        TrustLevel = "User"
	TrustNone        TrustLevel = "No trust"
)

const CtxTrust = "trust"

func SetTrust(ctx context.Context, account *Account) (context.Context, error) {
	if err := account.Validate(); err != nil {
		return nil, fmt.Errorf("cannot set trust because account is invalid: %w", err)
	}

	return context.WithValue(ctx, CtxTrust, account), nil
}

func SetTrustInGinContext(c *gin.Context, account *Account) error {
	ctx, err := SetTrust(c.Request.Context(), account)
	if err != nil {
		return err
	}
	c.Request = c.Request.WithContext(ctx)
	return nil
}

func GetTrustFromGinContext(c *gin.Context) Account {
	return GetTrust(c.Request.Context())
}

func GetTrust(ctx context.Context) Account {
	rawVal := ctx.Value(CtxTrust)
	if rawVal != nil {
		val, ok := rawVal.(*Account)
		if ok {
			return *val
		}
	}

	return Account{
		Trust:  TrustNone,
		Name:   "",
		Scopes: []scope.Scope{},
	}
}
