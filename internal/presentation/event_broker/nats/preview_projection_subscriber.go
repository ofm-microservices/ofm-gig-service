package nats

import (
	"context"

	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type gigPreviewProjectionSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	cfg    config.NATSConfig
	mapr   app.GigEventMapper
	log    logging.Logger
}

// NewGigPreviewProjectionSubscriber constructs the subscriber that seeds the
// freelancer preview cache after a gig is published.
func NewGigPreviewProjectionSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	mapr app.GigEventMapper,
	cfg config.NATSConfig,
	log logging.Logger,
) (GigPreviewProjectionSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if svc == nil {
		return nil, ErrNilGigService
	}
	if mapr == nil {
		return nil, ErrNilMessageMapper
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &gigPreviewProjectionSubscriber{
		broker: broker,
		svc:    svc,
		cfg:    cfg,
		mapr:   mapr,
		log:    log.With(logging.String("module", "gig-preview-projection-subscriber")),
	}, nil
}

// Subscribe starts the pull consumer that appends published gigs into the
// freelancer preview cache.
func (s *gigPreviewProjectionSubscriber) Subscribe(ctx context.Context) error {
	subject := s.cfg.GigPreviewProjectionSubject
	if subject == "" {
		subject = "gig.preview.projection.requested"
	}
	s.log.Info("registering gig preview projection pull consumer",
		logging.String("subject", subject),
		logging.String("stream", s.cfg.GigEventsStream),
		logging.Int("batch_size", s.cfg.GigProjectionBatchSize),
		logging.Any("max_wait", s.cfg.GigProjectionMaxWait),
		logging.Int("workers", s.cfg.GigProjectionWorkers),
	)

	return s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(),
		s.handleGigPreviewProjectionRequestedEvent,
	)
}

func (s *gigPreviewProjectionSubscriber) buildPullConsumerConfig() config.PullConsumerConfig {
	subject := s.cfg.GigPreviewProjectionSubject
	if subject == "" {
		subject = "gig.preview.projection.requested"
	}
	durable := s.cfg.GigPreviewProjectionDurable
	if durable == "" {
		durable = "gig_service_gig_preview_projection"
	}
	return config.PullConsumerConfig{
		Stream:     s.cfg.GigEventsStream,
		Subject:    subject,
		Durable:    durable,
		BatchSize:  s.cfg.GigProjectionBatchSize,
		MaxWait:    s.cfg.GigProjectionMaxWait,
		Workers:    s.cfg.GigProjectionWorkers,
		QueueSize:  s.cfg.GigProjectionQueueSize,
		AckWait:    s.cfg.GigProjectionAckWait,
		MaxDeliver: s.cfg.GigProjectionMaxDeliver,
		Adaptive: config.PullAdaptiveConfig{
			Enabled:         s.cfg.GigProjectionAdaptiveEnabled,
			CheckInterval:   s.cfg.GigProjectionAdaptiveCheckInterval,
			MediumPending:   s.cfg.GigProjectionAdaptiveMediumPending,
			HighPending:     s.cfg.GigProjectionAdaptiveHighPending,
			LowBatchSize:    s.cfg.GigProjectionAdaptiveLowBatchSize,
			LowMaxWait:      s.cfg.GigProjectionAdaptiveLowMaxWait,
			MediumBatchSize: s.cfg.GigProjectionAdaptiveMediumBatchSize,
			MediumMaxWait:   s.cfg.GigProjectionAdaptiveMediumMaxWait,
			HighBatchSize:   s.cfg.GigProjectionAdaptiveHighBatchSize,
			HighMaxWait:     s.cfg.GigProjectionAdaptiveHighMaxWait,
		},
	}
}
