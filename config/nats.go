package config

import "time"

// NATSConfig defines NATS streams, subjects, and pull-consumer settings used
// by gig-service.
type NATSConfig struct {
	URL      string `env:"NATS_URL,required"`
	User     string `env:"NATS_USER"`
	Password string `env:"NATS_PASSWORD"`

	GigEventsStream               string `env:"NATS_STREAM_GIG_EVENTS" envDefault:"GIG_EVENTS"`
	OrderEventsStream             string `env:"NATS_STREAM_ORDER_EVENTS" envDefault:"ORDER_EVENTS"`
	GigPublishedSubject           string `env:"NATS_SUBJECT_GIG_PUBLISHED" envDefault:"gig.published"`
	GigProjectionSubject          string `env:"NATS_SUBJECT_GIG_PROJECTION" envDefault:"gig.projection.requested"`
	GigPreviewProjectionSubject   string `env:"NATS_SUBJECT_GIG_PREVIEW_PROJECTION" envDefault:"gig.preview.projection.requested"`
	GigViewedSubject              string `env:"NATS_SUBJECT_GIG_VIEWED" envDefault:"gig.viewed"`
	ReviewEventsStream            string `env:"NATS_STREAM_REVIEW_EVENTS" envDefault:"REVIEW_EVENTS"`
	ReviewGigRatingSubject        string `env:"NATS_SUBJECT_REVIEW_GIG_RATING_REQUESTED" envDefault:"review.rating.gig"`
	OrderEventsOrderFundedSubject string `env:"NATS_SUBJECT_ORDER_FUNDED" envDefault:"order.funded"`

	GigProjectionDurable                 string        `env:"NATS_DURABLE_GIG_PROJECTION" envDefault:"gig_service_gig_projection"`
	GigPreviewProjectionDurable          string        `env:"NATS_DURABLE_GIG_PREVIEW_PROJECTION" envDefault:"gig_service_gig_preview_projection"`
	ReviewGigRatingDurable               string        `env:"NATS_DURABLE_REVIEW_GIG_RATING" envDefault:"gig_service_review_gig_rating"`
	OrderEventsOrderFundedDurable        string        `env:"NATS_DURABLE_ORDER_FUNDED" envDefault:"gig_service_order_funded"`
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
