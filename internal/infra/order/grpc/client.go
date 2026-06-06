package grpc

import (
	"context"
	"strings"

	"gig-service/config"
	app "gig-service/internal/application"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	orderwritev1 "github.com/ofm-microservices/ofm-common/proto/orderwrite/v1"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	conn *grpcpkg.ClientConn
	cl   orderwritev1.OrderWriteServiceClient
	mapr OrderCountMapper
	log  logging.Logger
}

// NewOrderCountService constructs the gRPC client used by gig-service to load
// order counts from order-service.
func NewOrderCountService(cfg config.OrderServiceConfig, log logging.Logger) (OrderCountService, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, ErrEmptyOrderServiceAddr
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
		cl:   orderwritev1.NewOrderWriteServiceClient(conn),
		mapr: newOrderCountMapper(),
		log:  log.With(logging.String("module", "grpc-order-client"), logging.String("address", cfg.Address)),
	}, nil
}

func (c *client) GetOrderCountByGigID(ctx context.Context, gigID string) (*app.OrderCountResult, error) {
	res, err := c.cl.GetOrderCountByGigID(ctx, c.mapr.ToGetOrderCountByGigIDRequest(strings.TrimSpace(gigID)))
	if err != nil {
		return nil, err
	}
	return c.mapr.ToGetOrderCountByGigIDResponse(res), nil
}

// Close closes the underlying gRPC connection.
func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.log.Info("closing order service grpc client")
	return c.conn.Close()
}
