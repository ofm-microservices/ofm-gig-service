package appfx

import (
	"context"

	"gig-service/config"
	filegrpc "gig-service/internal/infra/file/grpc"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// FileClientModule wires the upstream file-service client into gig-service.
var FileClientModule = fx.Options(
	fx.Provide(ProvideFileServiceClient),
)

// ProvideFileServiceClient constructs the upstream file-service gRPC client.
func ProvideFileServiceClient(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (filegrpc.FileService, error) {
	client, err := filegrpc.NewFileService(cfg.FileService, lg)
	if err != nil {
		lg.Error("connect file service failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
