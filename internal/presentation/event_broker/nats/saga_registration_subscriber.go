package nats

import (
	"context"

	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"

	"github.com/ofm-microseervices/ofm-common/pkg/logging"
)

type gigProjectionSubscriber struct {
	broker eventbroker.EventBroker
	writer ProjectionWriter
	cfg    config.NATSConfig
	mapr   app.GigEventMapper
	log    logging.Logger
}

// NewGigProjectionSubscriber constructs the subscriber that projects gig
// events into Redis.
func NewGigProjectionSubscriber(
	broker eventbroker.EventBroker,
	writer ProjectionWriter,
	mapr app.GigEventMapper,
	cfg config.NATSConfig,
	log logging.Logger,
) (GigProjectionSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if writer == nil {
		return nil, ErrNilProjectionWriter
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &gigProjectionSubscriber{
		broker: broker,
		writer: writer,
		cfg:    cfg,
		mapr:   mapr,
		log:    log.With(logging.String("module", "gig-projection-subscriber")),
	}, nil
}

// Subscribe starts the pull consumer that projects gig events into Redis.
func (s *gigProjectionSubscriber) Subscribe(ctx context.Context) error {
	s.log.Info("registering gig projection pull consumer",
		logging.String("subject", s.cfg.GigPublishedSubject),
		logging.String("stream", s.cfg.GigEventsStream),
		logging.Int("batch_size", s.cfg.GigProjectionBatchSize),
		logging.Any("max_wait", s.cfg.GigProjectionMaxWait),
		logging.Int("workers", s.cfg.GigProjectionWorkers),
	)

	return s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(),
		s.handleGigPublishedEvent,
	)
}

func (s *gigProjectionSubscriber) buildPullConsumerConfig() config.PullConsumerConfig {
	return config.PullConsumerConfig{
		Stream:     s.cfg.GigEventsStream,
		Subject:    s.cfg.GigPublishedSubject,
		Durable:    s.cfg.GigProjectionDurable,
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
