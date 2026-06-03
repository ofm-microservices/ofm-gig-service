package clickhouse

import (
	"context"
	app "gig-service/internal/application"
)

// PopularityRow aliases the application-level popularity aggregate shape used
// to materialize one gig popularity snapshot.
type PopularityRow = app.PopularityRow

// Client exposes the ClickHouse popularity aggregation source.
type Client interface {
	ListPopularityRows(ctx context.Context) ([]app.PopularityRow, error)
	Close() error
}
