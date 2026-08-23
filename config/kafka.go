package config

// KafkaConfig defines the Kafka broker and consumer group used by gig-service.
type KafkaConfig struct {
	Brokers                   []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"127.0.0.1:9092"`
	GroupID                   string   `env:"KAFKA_GIG_GROUP_ID" envDefault:"gig-service"`
	GigProjectionTopic        string   `env:"KAFKA_GIG_PROJECTION_TOPIC" envDefault:"gig.projection.requested"`
	GigPreviewProjectionTopic string   `env:"KAFKA_GIG_PREVIEW_PROJECTION_TOPIC" envDefault:"gig.preview.projection.requested"`
	ReviewRatingTopic         string   `env:"KAFKA_GIG_REVIEW_RATING_TOPIC" envDefault:"review.rating.gig"`
	OrderFundedTopic          string   `env:"KAFKA_GIG_ORDER_FUNDED_TOPIC" envDefault:"order.funded"`
	DeadLetterTopic           string   `env:"KAFKA_GIG_DLQ_TOPIC" envDefault:"gig-service.dead-letter"`
}
