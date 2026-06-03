package config

import "time"

// ClickHouseConfig defines the analytics source used to materialize gig
// popularity snapshots.
type ClickHouseConfig struct {
	Endpoint      string        `env:"CLICKHOUSE_ENDPOINT,required"`
	User          string        `env:"CLICKHOUSE_USER,required"`
	Password      string        `env:"CLICKHOUSE_PASSWORD,required"`
	Database      string        `env:"CLICKHOUSE_DATABASE" envDefault:"default"`
	Table         string        `env:"CLICKHOUSE_TABLE" envDefault:"ofm_business_events"`
	RefreshPeriod time.Duration `env:"CLICKHOUSE_REFRESH_PERIOD" envDefault:"5m"`
}
