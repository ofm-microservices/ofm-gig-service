package appfx

import (
	"context"

	"gig-service/config"
	ordergrpc "gig-service/internal/infra/order/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"go.uber.org/fx"
)

// OrderClientModule wires the upstream order-service client into gig-service.
var OrderClientModule = fx.Options(
	fx.Provide(ProvideOrderCountService),
)

// ProvideOrderCountService constructs the upstream order-service gRPC client.
func ProvideOrderCountService(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (ordergrpc.OrderCountService, error) {
	client, err := ordergrpc.NewOrderCountService(cfg.OrderService, lg)
	if err != nil {
		lg.Error("connect order service failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
