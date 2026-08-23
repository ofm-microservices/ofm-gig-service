package appfx

import (
	"context"
	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"
	kafkaevents "gig-service/internal/presentation/event_broker/kafka"
	grpcserver "gig-service/internal/presentation/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// PresentationModule wires transport adapters and background subscribers into
// the FX lifecycle.
var PresentationModule = fx.Options(
	fx.Provide(
		ProvideGigProjectionSubscriber,
		ProvideGigPreviewProjectionSubscriber,
		ProvideGigReviewRatingSubscriber,
		ProvideGigOrderFundedSubscriber,
		ProvideGRPCServer,
	),
	fx.Invoke(
		InvokeSubscribeGigProjection,
		InvokeSubscribeGigPreviewProjection,
		InvokeSubscribeGigReviewRating,
		InvokeSubscribeGigOrderFunded,
		InvokeWarmupSellerLookups,
		InvokeRunPopularityMaterializer,
		InvokeRunGRPCServer,
	),
)

// ProvideGigProjectionSubscriber constructs the Kafka subscriber that projects
// published gig events into Redis.
func ProvideGigProjectionSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	writer kafkaevents.ProjectionWriter,
	mapr app.GigEventMapper,
	cfg *config.Config,
	lg logging.Logger,
) (kafkaevents.GigProjectionSubscriber, error) {
	return kafkaevents.NewGigProjectionSubscriber(broker, svc, writer, mapr, cfg.Kafka, lg)
}

// ProvideGigPreviewProjectionSubscriber constructs the Kafka subscriber that
// seeds the freelancer preview cache after publish.
func ProvideGigPreviewProjectionSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	mapr app.GigEventMapper,
	cfg *config.Config,
	lg logging.Logger,
) (kafkaevents.GigPreviewProjectionSubscriber, error) {
	return kafkaevents.NewGigPreviewProjectionSubscriber(broker, svc, mapr, cfg.Kafka, lg)
}

// ProvideGigReviewRatingSubscriber constructs the Kafka subscriber that refreshes
// owner gig previews after review rating updates.
func ProvideGigReviewRatingSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	cfg *config.Config,
	lg logging.Logger,
) (kafkaevents.GigReviewRatingSubscriber, error) {
	return kafkaevents.NewGigReviewRatingSubscriber(broker, svc, cfg.Kafka, lg)
}

// ProvideGigOrderFundedSubscriber constructs the Kafka subscriber that
// refreshes owner gig previews after the canonical post-payment order event.
func ProvideGigOrderFundedSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	cfg *config.Config,
	lg logging.Logger,
) (kafkaevents.GigOrderCountSubscriber, error) {
	return kafkaevents.NewGigOrderFundedSubscriber(broker, svc, cfg.Kafka, lg)
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
	subscriber kafkaevents.GigProjectionSubscriber,
	cfg *config.Config,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel
			go func() {
				if err := subscriber.Subscribe(runCtx); err != nil && runCtx.Err() == nil {
					lg.Error("subscribe to gig projection failed", logging.Err(err))
				}
			}()

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

// InvokeSubscribeGigPreviewProjection starts the gig preview projection pull
// consumer.
func InvokeSubscribeGigPreviewProjection(
	lc fx.Lifecycle,
	subscriber kafkaevents.GigPreviewProjectionSubscriber,
	cfg *config.Config,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			go func() {
				if err := subscriber.Subscribe(runCtx); err != nil && runCtx.Err() == nil {
					lg.Error("subscribe to gig preview projection failed", logging.Err(err))
				}
			}()
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

// InvokeSubscribeGigReviewRating starts the gig review rating pull consumer.
func InvokeSubscribeGigReviewRating(
	lc fx.Lifecycle,
	subscriber kafkaevents.GigReviewRatingSubscriber,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			go func() {
				if err := subscriber.Subscribe(runCtx); err != nil && runCtx.Err() == nil {
					lg.Error("subscribe to gig review rating failed", logging.Err(err))
				}
			}()
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

// InvokeSubscribeGigOrderFunded starts the gig order count pull consumer.
func InvokeSubscribeGigOrderFunded(
	lc fx.Lifecycle,
	subscriber kafkaevents.GigOrderCountSubscriber,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			go func() {
				if err := subscriber.Subscribe(runCtx); err != nil && runCtx.Err() == nil {
					lg.Error("subscribe to gig order funded failed", logging.Err(err))
				}
			}()
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
