package appfx

import (
	"context"
	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"
	events "gig-service/internal/presentation/event_broker/nats"
	grpcserver "gig-service/internal/presentation/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// PresentationModule wires transport adapters and background subscribers into
// the FX lifecycle.
var PresentationModule = fx.Options(
	fx.Provide(
		ProvideGigProjectionSubscriber,
		ProvideGRPCServer,
	),
	fx.Invoke(
		InvokeSubscribeGigProjection,
		InvokeRunGRPCServer,
	),
)

// ProvideGigProjectionSubscriber constructs the NATS subscriber that projects
// published gig events into Redis.
func ProvideGigProjectionSubscriber(
	broker eventbroker.EventBroker,
	writer events.ProjectionWriter,
	mapr app.GigEventMapper,
	cfg *config.Config,
	lg logging.Logger,
) (events.GigProjectionSubscriber, error) {
	return events.NewGigProjectionSubscriber(broker, writer, mapr, cfg.NATS, lg)
}

// ProvideGRPCServer constructs the gRPC draft workflow server exposed by
// gig-service.
func ProvideGRPCServer(
	service app.GigService,
	cfg *config.Config,
	lg logging.Logger,
) (grpcserver.Server, error) {
	return grpcserver.NewServer(service, cfg.GRPC, lg)
}

// InvokeSubscribeGigProjection starts the gig projection pull consumer.
func InvokeSubscribeGigProjection(
	lc fx.Lifecycle,
	subscriber events.GigProjectionSubscriber,
	cfg *config.Config,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			if err := subscriber.Subscribe(runCtx); err != nil {
				lg.Error("subscribe to gig projection failed", logging.Err(err))
				cancel()
				return err
			}

			lg.Info("gig-service initialized", logging.String("env", cfg.App.Env))
			return nil
		},
		OnStop: func(context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
}

// InvokeRunGRPCServer starts and gracefully stops the gRPC server with the FX
// lifecycle.
func InvokeRunGRPCServer(lc fx.Lifecycle, srv grpcserver.Server) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.Start(); err != nil {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}
