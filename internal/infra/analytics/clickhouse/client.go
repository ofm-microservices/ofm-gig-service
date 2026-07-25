package clickhouse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gig-service/config"
	app "gig-service/internal/application"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type client struct {
	http *http.Client
	cfg  config.ClickHouseConfig
	log  logging.Logger
}

// New constructs the ClickHouse analytics reader used to materialize gig
// popularity scores.
func New(cfg config.ClickHouseConfig, log logging.Logger) (Client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("empty clickhouse endpoint")
	}
	if strings.TrimSpace(cfg.User) == "" {
		return nil, fmt.Errorf("empty clickhouse user")
	}
	if log == nil {
		return nil, fmt.Errorf("nil logger")
	}

	return &client{
		http: &http.Client{},
		cfg:  cfg,
		log:  log.With(logging.String("module", "clickhouse-client"), logging.String("endpoint", cfg.Endpoint)),
	}, nil
}

func (c *client) ListPopularityRows(ctx context.Context) ([]app.PopularityRow, error) {
	query := `
SELECT
    gig_id,
    countIf(operation = 'order.completed' AND timestamp >= now() - INTERVAL 30 DAY) AS completed_orders_last_30d,
    countIf(operation = 'review.create' AND timestamp >= now() - INTERVAL 30 DAY) AS reviews_count_last_30d,
    countIf(operation = 'gig.viewed' AND timestamp >= now() - INTERVAL 7 DAY) AS views_last_7d
FROM ` + c.cfg.Table + `
WHERE gig_id != ''
  AND (
    (operation = 'order.completed' AND timestamp >= now() - INTERVAL 30 DAY) OR
    (operation = 'review.create' AND timestamp >= now() - INTERVAL 30 DAY) OR
    (operation = 'gig.viewed' AND timestamp >= now() - INTERVAL 7 DAY)
  )
GROUP BY gig_id
`
	u, err := url.Parse(c.cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("default_format", "JSONEachRow")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.User, c.cfg.Password)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clickhouse query failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	rows := make([]app.PopularityRow, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row app.PopularityRow
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *client) Close() error { return nil }
