package audit

import (
	"context"
	"time"

	"github.com/ing-bank/golibs/pkg/access"
)

type Audit struct {
	Timestamp   time.Time `json:"timestamp"`
	RequestedBy string    `json:"requestedBy" binding:"required,corporatekey" validate:"required" example:"OF87UQ"`
	Source      string    `json:"source" binding:"required"`
	Controller  string    `json:"controller" binding:"required"`
}

func New(ctx context.Context, controllerName string) Audit {
	return Audit{
		Timestamp:   time.Now(),
		RequestedBy: access.GetTrust(ctx).Name, // TODO: we need to get an extra header
		Source:      access.GetTrust(ctx).Name,
		Controller:  controllerName,
	}
}
