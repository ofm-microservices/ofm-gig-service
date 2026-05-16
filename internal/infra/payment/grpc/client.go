package grpc

import (
	"context"

	"gig-service/config"
	app "gig-service/internal/application"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	paymentconnectv1 "github.com/ofm-microservices/ofm-common/proto/paymentconnect/v1"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	conn *grpcpkg.ClientConn
	cl   paymentconnectv1.PaymentOnboardingServiceClient
	mapr PaymentMapper
	log  logging.Logger
}

// NewPaymentService constructs the gRPC client used by gig-service to check
// freelancer Stripe Connect onboarding state.
func NewPaymentService(cfg config.PaymentServiceConfig, log logging.Logger) (PaymentService, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyPaymentServiceAddr
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	conn, err := grpcpkg.NewClient(
		cfg.Address,
		grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
		grpcpkg.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpcpkg.WithUnaryInterceptor(metrics.UnaryClientInterceptor()),
	)
	if err != nil {
		return nil, err
	}

	return &client{
		conn: conn,
		cl:   paymentconnectv1.NewPaymentOnboardingServiceClient(conn),
		mapr: newPaymentMapper(),
		log:  log.With(logging.String("module", "grpc-payment-client"), logging.String("address", cfg.Address)),
	}, nil
}

func (c *client) GetConnectStatus(ctx context.Context, userID string) (*app.ConnectStatusResult, error) {
	res, err := c.cl.GetConnectStatus(ctx, c.mapr.ToGetConnectStatusRequest(userID))
	if err != nil {
		return nil, c.mapr.ToError(err)
	}

	return c.mapr.ToGetConnectStatusResponse(res), nil
}

// Close closes the underlying gRPC connection.
func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.log.Info("closing payment service grpc client")
	return c.conn.Close()
}
