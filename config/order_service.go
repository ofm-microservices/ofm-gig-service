package config

// OrderServiceConfig defines the upstream order-service gRPC connection used
// to backfill gig order counts during preview projection rebuilds.
type OrderServiceConfig struct {
	Address string `env:"ORDER_SERVICE_ADDRESS,required"`
}
