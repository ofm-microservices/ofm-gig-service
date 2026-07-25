package config

// ReviewServiceConfig defines the upstream review-service gRPC connection used
// to backfill gig rating aggregates during preview projection rebuilds.
type ReviewServiceConfig struct {
	Address string `env:"REVIEW_SERVICE_ADDRESS,required"`
}
