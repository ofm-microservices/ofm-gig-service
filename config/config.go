package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config groups the full gig-service runtime configuration.
type Config struct {
	App            AppConfig
	DB             DBConfig
	GRPC           GRPCConfig
	Preview        PreviewPaginationConfig
	Metrics        MetricsConfig
	Tracing        TracingConfig
	Redis          RedisConfig
	NATS           NATSConfig
	FileService    FileServiceConfig
	PaymentService PaymentServiceConfig
	ReviewService  ReviewServiceConfig
	OrderService   OrderServiceConfig
	UserService    UserServiceConfig
	ClickHouse     ClickHouseConfig
}

// Load reads environment variables into Config and applies defaults.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, WrapParseEnvConfigError(err)
	}
	if err := validatePreviewPaginationConfig(cfg.Preview); err != nil {
		return nil, err
	}

	return cfg, nil
}
