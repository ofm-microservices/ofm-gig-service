package nats

import (
	"context"
	"gig-service/config"
	"gig-service/internal/domain"
	eventbroker "gig-service/internal/presentation/event_broker"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
)

// GigProjectionSubscriber consumes gig lifecycle events from NATS and writes
// the gig read model.
type GigProjectionSubscriber interface {
	Subscribe(ctx context.Context) error
}

// ProjectionWriter stores projected gig read models.
type ProjectionWriter interface {
	Upsert(ctx context.Context, gig *domain.Gig) error
	DeleteByID(ctx context.Context, gigID string) error
}

// PullConsumerConfigValidator validates pull-consumer runtime configuration
// before the broker touches JetStream state.
type PullConsumerConfigValidator interface {
	Validate(cfg config.PullConsumerConfig) error
}

// PullConsumerRuntime represents one configured JetStream pull-consumer
// runtime.
type PullConsumerRuntime interface {
	Start(ctx context.Context)
}

// PullConsumerRuntimeFactory builds the runtime used by the broker after the
// config is validated.
type PullConsumerRuntimeFactory interface {
	Create(
		nc *nats.Conn,
		log logging.Logger,
		cfg config.PullConsumerConfig,
		handler eventbroker.MessageHandler,
	) (PullConsumerRuntime, error)
}
