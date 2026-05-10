package appfx

import (
	"context"
	"gig-service/config"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// LoggerModule provides the structured logger used across gig-service.
var LoggerModule = fx.Options(
	fx.Provide(ProvideLogger),
)

// ProvideLogger builds the service logger from runtime configuration.
func ProvideLogger(lc fx.Lifecycle, cfg *config.Config) (logging.Logger, error) {
	lg, err := logging.New("gig-service", cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return lg.Sync()
		},
	})

	return lg, nil
}
