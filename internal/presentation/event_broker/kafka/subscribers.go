package kafka

import (
	"context"
	"encoding/json"
	"gig-service/config"
	app "gig-service/internal/application"
	eventbroker "gig-service/internal/presentation/event_broker"
	"strings"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	orderflowv1 "github.com/ofm-microservices/ofm-common/proto/orderflow/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type gigProjectionSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	writer ProjectionWriter
	mapr   app.GigEventMapper
	log    logging.Logger
	topic  string
}

type gigReviewRatingSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	log    logging.Logger
	topic  string
}

// NewGigReviewRatingSubscriber constructs the Kafka subscriber that refreshes
// cached gig ratings after review events.
func NewGigReviewRatingSubscriber(broker eventbroker.EventBroker, svc app.GigService, cfg config.KafkaConfig, log logging.Logger) (GigReviewRatingSubscriber, error) {
	if broker == nil || svc == nil || log == nil {
		return nil, ErrInvalidSubscriberDependency
	}
	topic := cfg.ReviewRatingTopic
	if strings.TrimSpace(topic) == "" {
		topic = "review.rating.gig"
	}
	return &gigReviewRatingSubscriber{broker: broker, svc: svc, log: log.With(logging.String("module", "kafka-gig-review-rating-subscriber")), topic: topic}, nil
}

func (s *gigReviewRatingSubscriber) Subscribe(ctx context.Context) error {
	return s.broker.Subscribe(ctx, s.topic, s.handle)
}

func (s *gigReviewRatingSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	var event struct {
		GigID string `json:"gig_id"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}
	if strings.TrimSpace(event.GigID) == "" {
		return ErrInvalidGigPayload
	}
	return s.svc.RefreshPreviewRating(ctx, event.GigID)
}

type gigOrderCountSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	log    logging.Logger
	topic  string
}

// NewGigOrderFundedSubscriber constructs the Kafka subscriber that refreshes
// cached gig order counts after successful funded-order events.
func NewGigOrderFundedSubscriber(broker eventbroker.EventBroker, svc app.GigService, cfg config.KafkaConfig, log logging.Logger) (GigOrderCountSubscriber, error) {
	if broker == nil || svc == nil || log == nil {
		return nil, ErrInvalidSubscriberDependency
	}
	topic := cfg.OrderFundedTopic
	if strings.TrimSpace(topic) == "" {
		topic = "order.funded"
	}
	return &gigOrderCountSubscriber{broker: broker, svc: svc, log: log.With(logging.String("module", "kafka-gig-order-funded-subscriber")), topic: topic}, nil
}

func (s *gigOrderCountSubscriber) Subscribe(ctx context.Context) error {
	return s.broker.Subscribe(ctx, s.topic, s.handle)
}

func (s *gigOrderCountSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	var event orderflowv1.OrderSagaResult
	if err := protojson.Unmarshal(payload, &event); err != nil {
		s.log.Warn("skip malformed order funded payload", logging.Err(err))
		return nil
	}
	if strings.TrimSpace(event.GigId) == "" || strings.TrimSpace(event.Status) != "success" {
		return nil
	}
	return s.svc.RefreshPreviewOrderCount(ctx, event.GigId)
}

// NewGigProjectionSubscriber constructs the Kafka subscriber for the gig
// service-owned Redis projection.
func NewGigProjectionSubscriber(broker eventbroker.EventBroker, svc app.GigService, writer ProjectionWriter, mapr app.GigEventMapper, cfg config.KafkaConfig, log logging.Logger) (GigProjectionSubscriber, error) {
	if broker == nil || svc == nil || writer == nil || mapr == nil || log == nil {
		return nil, ErrInvalidSubscriberDependency
	}
	topic := cfg.GigProjectionTopic
	if strings.TrimSpace(topic) == "" {
		topic = "gig.projection.requested"
	}
	return &gigProjectionSubscriber{broker: broker, svc: svc, writer: writer, mapr: mapr, log: log.With(logging.String("module", "kafka-gig-projection-subscriber")), topic: topic}, nil
}

func (s *gigProjectionSubscriber) Subscribe(ctx context.Context) error {
	return s.broker.Subscribe(ctx, s.topic, s.handle)
}

func (s *gigProjectionSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	gig, err := s.mapr.FromPublishedPayload(payload)
	if err != nil {
		return err
	}
	if gig == nil || strings.TrimSpace(gig.ID) == "" {
		return ErrInvalidGigPayload
	}
	projected, err := s.svc.Project(ctx, gig)
	if err != nil {
		return err
	}
	return s.writer.Upsert(ctx, projected)
}

type gigPreviewProjectionSubscriber struct {
	broker eventbroker.EventBroker
	svc    app.GigService
	mapr   app.GigEventMapper
	log    logging.Logger
	topic  string
}

// NewGigPreviewProjectionSubscriber constructs the Kafka subscriber for
// freelancer preview projection after a gig is published.
func NewGigPreviewProjectionSubscriber(broker eventbroker.EventBroker, svc app.GigService, mapr app.GigEventMapper, cfg config.KafkaConfig, log logging.Logger) (GigPreviewProjectionSubscriber, error) {
	if broker == nil || svc == nil || mapr == nil || log == nil {
		return nil, ErrInvalidSubscriberDependency
	}
	topic := cfg.GigPreviewProjectionTopic
	if strings.TrimSpace(topic) == "" {
		topic = "gig.preview.projection.requested"
	}
	return &gigPreviewProjectionSubscriber{broker: broker, svc: svc, mapr: mapr, log: log.With(logging.String("module", "kafka-gig-preview-projection-subscriber")), topic: topic}, nil
}

func (s *gigPreviewProjectionSubscriber) Subscribe(ctx context.Context) error {
	return s.broker.Subscribe(ctx, s.topic, s.handle)
}

func (s *gigPreviewProjectionSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	gig, err := s.mapr.FromPublishedPayload(payload)
	if err != nil {
		return err
	}
	if gig == nil || strings.TrimSpace(gig.ID) == "" {
		return ErrInvalidGigPayload
	}
	projected, err := s.svc.Project(ctx, gig)
	if err != nil {
		return err
	}
	if projected == nil {
		return ErrInvalidGigPayload
	}
	return s.svc.AppendPreviewGig(ctx, projected)
}
