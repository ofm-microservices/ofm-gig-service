package appfx

import (
	"context"
	"gig-service/config"
	eventbroker "gig-service/internal/presentation/event_broker"
	broker "gig-service/internal/presentation/event_broker/nats"
	natsbootstrap "gig-service/pkg/messaging/nats"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// MessagingModule wires NATS bootstrap and broker runtime into gig-service.
var MessagingModule = fx.Options(
	fx.Invoke(InvokeEnsureStream),
	fx.Provide(ProvideEventBroker),
)

var ensureStream = natsbootstrap.EnsureStream
var newEventBroker = broker.NewBroker

// InvokeEnsureStream ensures the JetStream streams gig-service depends on.
func InvokeEnsureStream(cfg *config.Config, lg logging.Logger) error {
	if err := ensureStream(cfg.NATS, lg); err != nil {
		lg.Error("bootstrap jetstream resources failed", logging.Err(err))
		return err
	}

	return nil
}

// ProvideEventBroker constructs the concrete NATS event broker.
func ProvideEventBroker(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (eventbroker.EventBroker, error) {
	eventBroker, err := newEventBroker(cfg.NATS, lg)
	if err != nil {
		lg.Error("connect nats failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			eventBroker.Close()
			return nil
		},
	})

	return eventBroker, nil
}
