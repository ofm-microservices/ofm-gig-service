package appfx

import (
	"context"
	"gig-service/config"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	ch "gig-service/internal/infra/analytics/clickhouse"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"go.uber.org/fx"
	"time"
)

// AnalyticsModule wires the ClickHouse popularity reader used by gig-service.
var AnalyticsModule = fx.Options(
	fx.Provide(ProvideClickHouseClient),
)

// ProvideClickHouseClient constructs the ClickHouse analytics reader.
func ProvideClickHouseClient(cfg *config.Config, lg logging.Logger) (app.PopularitySource, error) {
	return ch.New(cfg.ClickHouse, lg)
}

// InvokeRunPopularityMaterializer starts the periodic popularity refresh job.
func InvokeRunPopularityMaterializer(lc fx.Lifecycle, repo domain.GigRepository, svc app.GigService, cfg *config.Config, lg logging.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				ticker := time.NewTicker(cfg.ClickHouse.RefreshPeriod)
				defer ticker.Stop()
				refresh := func() {
					ctx := context.Background()
					gigs, err := repo.ListAll(ctx)
					if err != nil {
						lg.Error("load gigs for popularity refresh failed", logging.Operation("gig.popularity.refresh"), logging.Err(err))
						return
					}
					if err := svc.RebuildPopularitySnapshots(ctx, gigs); err != nil {
						lg.Error("refresh gig popularity failed", logging.Operation("gig.popularity.refresh"), logging.Err(err))
					}
				}
				refresh()
				for range ticker.C {
					refresh()
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error { return nil },
	})
}
