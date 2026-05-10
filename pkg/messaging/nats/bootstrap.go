package nats

import (
	"gig-service/config"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"time"

	"github.com/nats-io/nats.go"
)

type bootstrapConn interface {
	JetStream() (jetStreamManager, error)
	Close()
}

type jetStreamManager interface {
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
}

type realBootstrapConn struct {
	*nats.Conn
}

func (c realBootstrapConn) JetStream() (jetStreamManager, error) {
	return c.Conn.JetStream()
}

var connectBootstrap = func(cfg config.NATSConfig) (bootstrapConn, error) {
	nc, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	return realBootstrapConn{Conn: nc}, nil
}

// EnsureStream creates or updates the JetStream streams required by
// gig-service.
func EnsureStream(cfg config.NATSConfig, log logging.Logger) error {
	if log == nil {
		return ErrNilLogger
	}

	lg := log.With(logging.String("module", "jetstream-bootstrap"))
	lg.Info("ensuring jetstream stream",
		logging.String("stream", cfg.GigEventsStream),
		logging.String("subject", cfg.GigPublishedSubject),
	)

	nc, err := connectBootstrap(cfg)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return WrapInitJetStreamContextError(err)
	}

	streamCfg := &nats.StreamConfig{
		Name:      cfg.GigEventsStream,
		Subjects:  []string{cfg.GigPublishedSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	_, err = js.AddStream(streamCfg)
	if err != nil {
		if _, updateErr := js.UpdateStream(streamCfg); updateErr != nil {
			return WrapEnsureStreamError(streamCfg.Name, err, updateErr)
		}
	}

	lg.Info("jetstream streams ensured",
		logging.String("gig_events_stream", streamCfg.Name),
	)
	return nil
}

// Connect establishes the low-level NATS connection used by bootstrap code.
func Connect(cfg config.NATSConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("gig-service"),
		nats.MaxReconnects(-1),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, WrapConnectToNATSError(err)
	}

	return nc, nil
}
