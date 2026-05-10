package appfx

import (
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	readrepo "gig-service/internal/infra/read/redis"
	writerepo "gig-service/internal/infra/write/yugabyte"
	events "gig-service/internal/presentation/event_broker/nats"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// RepoModule wires write- and read-model repositories into the FX graph.
var RepoModule = fx.Options(
	fx.Provide(
		writerepo.NewPgErrorTranslator,
		ProvideWriteRepo,
		ProvideReadRepo,
	),
)

// ProvideWriteRepo constructs the Yugabyte-backed gig repository.
func ProvideWriteRepo(dbx *sqlx.DB, translator writerepo.DBErrorTranslator) (domain.GigRepository, error) {
	return writerepo.New(dbx, translator)
}

// ProvideReadRepo constructs the Redis-backed gig read-model writer.
func ProvideReadRepo(rdb *redis.Client, mapr app.GigEventMapper) (events.ProjectionWriter, error) {
	return readrepo.New(rdb, mapr)
}
