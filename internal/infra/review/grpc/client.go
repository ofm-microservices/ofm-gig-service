package grpc

import (
	"context"

	"gig-service/config"
	app "gig-service/internal/application"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	reviewv1 "github.com/ofm-microservices/ofm-common/proto/review/v1"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	conn *grpcpkg.ClientConn
	cl   reviewv1.ReviewServiceClient
	mapr ReviewMapper
	log  logging.Logger
}

// NewReviewService constructs the gRPC client used by gig-service to load gig
// rating aggregates from review-service.
func NewReviewService(cfg config.ReviewServiceConfig, log logging.Logger) (ReviewService, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyReviewServiceAddr
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
		cl:   reviewv1.NewReviewServiceClient(conn),
		mapr: newReviewMapper(),
		log:  log.With(logging.String("module", "grpc-review-client"), logging.String("address", cfg.Address)),
	}, nil
}

func (c *client) GetGigRatingSummary(ctx context.Context, gigID string) (*app.ReviewSummary, error) {
	res, err := c.cl.GetGigRatingSummary(ctx, c.mapr.ToGetGigRatingSummaryRequest(gigID))
	if err != nil {
		return nil, err
	}
	return c.mapr.ToGetGigRatingSummaryResponse(res), nil
}

// Close closes the underlying gRPC connection.
func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.log.Info("closing review service grpc client")
	return c.conn.Close()
}
