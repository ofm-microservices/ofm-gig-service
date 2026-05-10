package config

import "time"

// NATSConfig defines NATS streams, subjects, and pull-consumer settings used
// by gig-service.
type NATSConfig struct {
	URL      string `env:"NATS_URL,required"`
	User     string `env:"NATS_USER"`
	Password string `env:"NATS_PASSWORD"`

	GigEventsStream     string `env:"NATS_STREAM_GIG_EVENTS" envDefault:"GIG_EVENTS"`
	GigPublishedSubject string `env:"NATS_SUBJECT_GIG_PUBLISHED" envDefault:"gig.published"`

	GigProjectionDurable                 string        `env:"NATS_DURABLE_GIG_PROJECTION" envDefault:"gig_service_gig_projection"`
	GigProjectionBatchSize               int           `env:"NATS_GIG_PROJECTION_BATCH_SIZE" envDefault:"100"`
	GigProjectionMaxWait                 time.Duration `env:"NATS_GIG_PROJECTION_MAX_WAIT" envDefault:"500ms"`
	GigProjectionWorkers                 int           `env:"NATS_GIG_PROJECTION_WORKERS" envDefault:"4"`
	GigProjectionQueueSize               int           `env:"NATS_GIG_PROJECTION_QUEUE_SIZE" envDefault:"200"`
	GigProjectionAckWait                 time.Duration `env:"NATS_GIG_PROJECTION_ACK_WAIT" envDefault:"30s"`
	GigProjectionMaxDeliver              int           `env:"NATS_GIG_PROJECTION_MAX_DELIVER" envDefault:"5"`
	GigProjectionAdaptiveEnabled         bool          `env:"NATS_GIG_PROJECTION_ADAPTIVE_ENABLED" envDefault:"false"`
	GigProjectionAdaptiveCheckInterval   time.Duration `env:"NATS_GIG_PROJECTION_ADAPTIVE_CHECK_INTERVAL" envDefault:"2s"`
	GigProjectionAdaptiveMediumPending   int           `env:"NATS_GIG_PROJECTION_ADAPTIVE_MEDIUM_PENDING" envDefault:"200"`
	GigProjectionAdaptiveHighPending     int           `env:"NATS_GIG_PROJECTION_ADAPTIVE_HIGH_PENDING" envDefault:"1000"`
	GigProjectionAdaptiveLowBatchSize    int           `env:"NATS_GIG_PROJECTION_ADAPTIVE_LOW_BATCH_SIZE" envDefault:"25"`
	GigProjectionAdaptiveLowMaxWait      time.Duration `env:"NATS_GIG_PROJECTION_ADAPTIVE_LOW_MAX_WAIT" envDefault:"1s"`
	GigProjectionAdaptiveMediumBatchSize int           `env:"NATS_GIG_PROJECTION_ADAPTIVE_MEDIUM_BATCH_SIZE" envDefault:"100"`
	GigProjectionAdaptiveMediumMaxWait   time.Duration `env:"NATS_GIG_PROJECTION_ADAPTIVE_MEDIUM_MAX_WAIT" envDefault:"500ms"`
	GigProjectionAdaptiveHighBatchSize   int           `env:"NATS_GIG_PROJECTION_ADAPTIVE_HIGH_BATCH_SIZE" envDefault:"300"`
	GigProjectionAdaptiveHighMaxWait     time.Duration `env:"NATS_GIG_PROJECTION_ADAPTIVE_HIGH_MAX_WAIT" envDefault:"100ms"`
}
