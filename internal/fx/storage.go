package appfx

import (
	"context"
	"gig-service/config"
	rdb "gig-service/pkg/storage/redis"
	ydb "gig-service/pkg/storage/postgres"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// StorageModule wires the gig-service database, Redis read-model store, and
// migrations into the FX graph.
var StorageModule = fx.Options(
	fx.Invoke(InvokeRunMigrations),
	fx.Provide(
		ProvidePostgresDB,
		ProvideRedisClient,
	),
)

var runMigrations = ydb.RunMigrations
var openPostgresDB = ydb.Open
var openRedisClient = rdb.Open

// InvokeRunMigrations applies the gig-service write-model migrations.
func InvokeRunMigrations(cfg *config.Config, lg logging.Logger) error {
	const attempts = 10
	for attempt := 1; attempt <= attempts; attempt++ {
		err := runMigrations(cfg.DB)
		if err == nil {
			lg.Info("migrations applied")
			return nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "deadlock") || attempt == attempts {
			lg.Error("run migrations failed", logging.Err(err))
			return err
		}
		backoff := time.Duration(attempt) * 2 * time.Second
		lg.Warn("migration lock contention; retrying", logging.Err(err), logging.String("retry_in", backoff.String()))
		time.Sleep(backoff)
	}
	return nil
}

// ProvidePostgresDB opens the PostgreSQL connection owned by gig-service.
func ProvidePostgresDB(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (*sqlx.DB, error) {
	dbx, err := openPostgresDB(cfg.DB)
	if err != nil {
		lg.Error("open database failed", logging.Err(err))
		return nil, err
	}

	lg.Info("database connected")

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			dbx.Close()
			return nil
		},
	})

	return dbx, nil
}

// ProvideRedisClient opens the Redis client used for the gig read model.
func ProvideRedisClient(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (*redis.Client, error) {
	client, err := openRedisClient(context.Background(), cfg.Redis)
	if err != nil {
		lg.Error("open redis failed", logging.Err(err))
		return nil, err
	}

	lg.Info("redis connected")

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
