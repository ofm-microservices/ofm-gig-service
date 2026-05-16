package appfx

import (
	"context"

	"gig-service/config"
	paymentgrpc "gig-service/internal/infra/payment/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// PaymentClientModule wires the upstream payment-service client into gig-service.
var PaymentClientModule = fx.Options(
	fx.Provide(ProvidePaymentServiceClient),
)

// ProvidePaymentServiceClient constructs the upstream payment-service gRPC client.
func ProvidePaymentServiceClient(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (paymentgrpc.PaymentService, error) {
	client, err := paymentgrpc.NewPaymentService(cfg.PaymentService, lg)
	if err != nil {
		lg.Error("connect payment service failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
