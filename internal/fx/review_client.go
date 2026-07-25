package appfx

import (
	"context"

	"gig-service/config"
	reviewgrpc "gig-service/internal/infra/review/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"go.uber.org/fx"
)

// ReviewClientModule wires the upstream review-service client into gig-service.
var ReviewClientModule = fx.Options(
	fx.Provide(ProvideReviewServiceClient),
)

// ProvideReviewServiceClient constructs the upstream review-service gRPC client.
func ProvideReviewServiceClient(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (reviewgrpc.ReviewService, error) {
	client, err := reviewgrpc.NewReviewService(cfg.ReviewService, lg)
	if err != nil {
		lg.Error("connect review service failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
