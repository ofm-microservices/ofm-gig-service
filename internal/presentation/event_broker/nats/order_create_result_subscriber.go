package nats

import (
	"context"
	"strings"

	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	orderflowv1 "github.com/ofm-microservices/ofm-common/proto/orderflow/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type gigOrderCreateResultSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	cfg    config.NATSConfig
	log    logging.Logger
}

// NewGigOrderFundedSubscriber constructs the subscriber that refreshes gig
// preview order counts from the canonical post-payment order event.
func NewGigOrderFundedSubscriber(
	broker eventbroker.EventBroker,
	svc app.GigService,
	cfg config.NATSConfig,
	log logging.Logger,
) (GigOrderCountSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if svc == nil {
		return nil, ErrNilGigService
	}
	if log == nil {
		return nil, ErrNilLogger
	}
	return &gigOrderCreateResultSubscriber{
		broker: broker,
		svc:    svc,
		cfg:    cfg,
		log:    log.With(logging.String("module", "gig-order-funded-subscriber")),
	}, nil
}

// Subscribe starts the pull consumer that refreshes cached gig order counts.
func (s *gigOrderCreateResultSubscriber) Subscribe(ctx context.Context) error {
	subject := s.cfg.OrderEventsOrderFundedSubject
	if subject == "" {
		subject = "order.funded"
	}
	s.log.Info("registering gig order count pull consumer",
		logging.String("subject", subject),
		logging.String("stream", s.cfg.OrderEventsStream),
		logging.Int("batch_size", s.cfg.GigProjectionBatchSize),
		logging.Any("max_wait", s.cfg.GigProjectionMaxWait),
		logging.Int("workers", s.cfg.GigProjectionWorkers),
	)
	return s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(),
		s.handle,
	)
}

func (s *gigOrderCreateResultSubscriber) buildPullConsumerConfig() config.PullConsumerConfig {
	subject := s.cfg.OrderEventsOrderFundedSubject
	if subject == "" {
		subject = "order.funded"
	}
	durable := s.cfg.OrderEventsOrderFundedDurable
	if durable == "" {
		durable = "gig_service_order_funded"
	}
	return config.PullConsumerConfig{
		Stream:     s.cfg.OrderEventsStream,
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

func (s *gigOrderCreateResultSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	var evt orderflowv1.OrderSagaResult
	if err := protojson.Unmarshal(payload, &evt); err != nil {
		s.log.Warn("skip malformed order funded payload",
			logging.Operation("gig.order_funded.skip_invalid"),
			logging.Err(err),
		)
		return nil
	}
	if strings.TrimSpace(evt.GigId) == "" {
		s.log.Warn("skip order funded payload without gig id",
			logging.Operation("gig.order_funded.skip_invalid"),
		)
		return nil
	}
	if strings.TrimSpace(evt.Status) != "success" {
		return nil
	}
	return s.svc.RefreshPreviewOrderCount(ctx, evt.GigId)
}
