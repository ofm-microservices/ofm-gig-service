package nats

import (
	"context"
	"encoding/json"

	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type gigReviewRatingSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	cfg    config.NATSConfig
	log    logging.Logger
}

type reviewRatingEvent struct {
	GigID string `json:"gig_id"`
}

// NewGigReviewRatingSubscriber constructs the subscriber that refreshes the
// gig preview cache rating fields after review aggregates change.
func NewGigReviewRatingSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	cfg config.NATSConfig,
	log logging.Logger,
) (GigReviewRatingSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if svc == nil {
		return nil, ErrNilGigService
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &gigReviewRatingSubscriber{
		broker: broker,
		svc:    svc,
		cfg:    cfg,
		log:    log.With(logging.String("module", "gig-review-rating-subscriber")),
	}, nil
}

// Subscribe starts the pull consumer that refreshes cached gig rating fields.
func (s *gigReviewRatingSubscriber) Subscribe(ctx context.Context) error {
	subject := s.cfg.ReviewGigRatingSubject
	if subject == "" {
		subject = "review.rating.gig"
	}
	s.log.Info("registering gig review rating pull consumer",
		logging.String("subject", subject),
		logging.String("stream", s.cfg.ReviewEventsStream),
		logging.Int("batch_size", s.cfg.GigProjectionBatchSize),
		logging.Any("max_wait", s.cfg.GigProjectionMaxWait),
		logging.Int("workers", s.cfg.GigProjectionWorkers),
	)
	return s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(),
		s.handleGigReviewRatingRequestedEvent,
	)
}

func (s *gigReviewRatingSubscriber) buildPullConsumerConfig() config.PullConsumerConfig {
	subject := s.cfg.ReviewGigRatingSubject
	if subject == "" {
		subject = "review.rating.gig"
	}
	durable := s.cfg.ReviewGigRatingDurable
	if durable == "" {
		durable = "gig_service_review_gig_rating"
	}
	return config.PullConsumerConfig{
		Stream:     s.cfg.ReviewEventsStream,
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

func (s *gigReviewRatingSubscriber) handleGigReviewRatingRequestedEvent(ctx context.Context, _ string, payload []byte) error {
	var evt reviewRatingEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return err
	}
	if evt.GigID == "" {
		return ErrInvalidGigPayload
	}
	return s.svc.RefreshPreviewRating(ctx, evt.GigID)
}
