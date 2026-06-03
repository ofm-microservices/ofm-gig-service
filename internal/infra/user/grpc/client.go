package grpc

import (
	"context"
	"strings"

	"gig-service/config"
	app "gig-service/internal/application"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	userv1 "github.com/ofm-microservices/ofm-common/proto/user/v1"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	conn *grpcpkg.ClientConn
	cl   userv1.UserQueryServiceClient
	mapr UserMapper
	log  logging.Logger
}

// NewUserPreviewClient constructs the gRPC client used by gig-service to
// backfill seller usernames from user-service.
func NewUserPreviewClient(cfg config.UserServiceConfig, log logging.Logger) (UserPreviewClient, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, ErrEmptyAddress
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
		cl:   userv1.NewUserQueryServiceClient(conn),
		mapr: newUserMapper(),
		log:  log.With(logging.String("module", "grpc-user-client"), logging.String("address", cfg.Address)),
	}, nil
}

func (c *client) GetUserPreviewByIDNoCache(ctx context.Context, userID string) (*app.UserPreview, error) {
	res, err := c.cl.GetUserPreviewByIDNoCache(ctx, c.mapr.ToGetUserPreviewByIDNoCacheRequest(strings.TrimSpace(userID)))
	if err != nil {
		return nil, c.mapr.ToError(err)
	}
	return c.mapr.ToGetUserPreviewByIDNoCacheResponse(res), nil
}

func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.log.Info("closing user service grpc client")
	return c.conn.Close()
}
