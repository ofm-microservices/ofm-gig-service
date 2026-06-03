package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	"github.com/redis/go-redis/v9"
)

type repo struct {
	rdb  *redis.Client
	mapr app.GigEventMapper
	log  logging.Logger
}

// New constructs the Redis-backed gig read-model repository.
func New(rdb *redis.Client, mapr app.GigEventMapper, log logging.Logger) (domain.GigReadRepository, error) {
	if rdb == nil {
		return nil, ErrNilRedisClient
	}
	if mapr == nil {
		return nil, ErrNilGig
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &repo{rdb: rdb, mapr: mapr, log: log.With(logging.String("module", "redis-repository"))}, nil
}

// Upsert stores the gig read model in Redis.
func (r *repo) Upsert(ctx context.Context, gig *domain.Gig) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("set", "gig", status, time.Since(started)) }()

	if gig == nil {
		status = "error"
		r.log.Error("upsert gig failed",
			logging.Operation("redis.gig.upsert"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.Err(ErrNilGig),
		)
		return ErrNilGig
	}

	payload, err := r.mapr.ToReadModelPayload(gig)
	if err != nil {
		status = "error"
		r.log.Error("upsert gig failed",
			logging.Operation("redis.gig.upsert"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gig.ID),
			logging.Err(err),
		)
		return WrapMarshalGigCacheError(err)
	}

	key := GigCacheKey(gig.ID)
	if err := r.rdb.Set(ctx, key, payload, 0).Err(); err != nil {
		status = "error"
		r.log.Error("upsert gig failed",
			logging.Operation("redis.gig.upsert"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gig.ID),
			logging.String("cache_key", key),
			logging.Err(err),
		)
		return WrapSetGigCacheError(key, err)
	}

	return nil
}

// GetByID loads the gig read model from Redis.
func (r *repo) GetByID(ctx context.Context, gigID string) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("get", "gig", status, time.Since(started)) }()

	key := GigCacheKey(gigID)
	raw, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		status = "error"
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrGigNotFound
		}
		r.log.Error("get gig failed",
			logging.Operation("redis.gig.get_by_id"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("cache_key", key),
			logging.Err(err),
		)
		return nil, WrapGetGigCacheError(key, err)
	}

	gig, err := r.mapr.FromReadModelPayload([]byte(raw))
	if err != nil {
		status = "error"
		r.log.Error("get gig failed",
			logging.Operation("redis.gig.get_by_id"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("cache_key", key),
			logging.Err(err),
		)
		return nil, WrapUnmarshalGigCacheError(err)
	}

	return gig, nil
}

// DeleteByID removes the gig read model from Redis.
func (r *repo) DeleteByID(ctx context.Context, gigID string) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("del", "gig", status, time.Since(started)) }()

	key := GigCacheKey(gigID)
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		status = "error"
		r.log.Error("delete gig failed",
			logging.Operation("redis.gig.delete_by_id"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("cache_key", key),
			logging.Err(err),
		)
		return WrapDeleteGigCacheError(key, err)
	}

	return nil
}

func (r *repo) UpsertPreviewWindow(ctx context.Context, userID string, window int, gigs []*domain.GigPreview, hasMore bool, ttl time.Duration) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("set", "gig_preview", status, time.Since(started)) }()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.ErrInvalidUsername
	}
	if window < 0 {
		window = 0
	}

	pipe := r.rdb.Pipeline()
	windowKey := GigPreviewWindowKey(userID, window)
	pipe.Del(ctx, windowKey)
	for _, gig := range gigs {
		if gig == nil {
			continue
		}
		itemPayload, marshalErr := json.Marshal(gig)
		if marshalErr != nil {
			status = "error"
			return marshalErr
		}
		pipe.Set(ctx, GigPreviewPayloadKey(gig.ID), itemPayload, ttl)
		pipe.ZAdd(ctx, windowKey, redis.Z{Score: float64(gigPopularityScore(gig)), Member: gig.ID})
	}
	if window > 0 && ttl > 0 {
		pipe.Expire(ctx, windowKey, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		status = "error"
		return err
	}

	return nil
}

func (r *repo) AppendPreviewGig(ctx context.Context, userID string, gig *domain.GigPreview, windowSize int, ttl time.Duration) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("set", "gig_preview", status, time.Since(started)) }()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.ErrInvalidUsername
	}
	if gig == nil || strings.TrimSpace(gig.ID) == "" {
		return ErrNilGig
	}
	if windowSize <= 0 {
		windowSize = 1
	}

	leaseKey := GigPreviewLeaseKey(userID)
	token := fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), gig.ID)
	ok, err := r.rdb.SetNX(ctx, leaseKey, token, 15*time.Second).Result()
	if err != nil {
		status = "error"
		return WrapPreviewLeaseError(leaseKey, err)
	}
	if !ok {
		status = "error"
		return ErrPreviewLeaseBusy
	}
	defer func() {
		released, relErr := r.releasePreviewLease(ctx, leaseKey, token)
		if relErr != nil {
			status = "error"
			r.log.Error("release preview lease failed",
				logging.Operation("redis.gig_preview.release_lease"),
				logging.String("lease_key", leaseKey),
				logging.Err(relErr),
			)
			return
		}
		if !released {
			r.log.Warn("preview lease release lost ownership",
				logging.Operation("redis.gig_preview.release_lease"),
				logging.String("lease_key", leaseKey),
			)
		}
	}()

	indexes, err := r.previewWindowIndexes(ctx, userID)
	if err != nil {
		status = "error"
		return err
	}

	if len(indexes) == 0 {
		return r.UpsertPreviewWindow(ctx, userID, 0, []*domain.GigPreview{gig}, false, 0)
	}

	tailWindow := indexes[len(indexes)-1]
	for _, window := range indexes {
		current, loadErr := r.ListPreviewWindow(ctx, userID, window)
		if loadErr != nil {
			if errors.Is(loadErr, domain.ErrGigNotFound) {
				continue
			}
			status = "error"
			return loadErr
		}
		for _, existing := range current.Gigs {
			if existing != nil && existing.ID == gig.ID {
				return nil
			}
		}
		if window == tailWindow && len(current.Gigs) < windowSize {
			next := append(append([]*domain.GigPreview(nil), current.Gigs...), gig)
			return r.UpsertPreviewWindow(ctx, userID, window, next, false, ttl)
		}
	}

	return r.UpsertPreviewWindow(ctx, userID, tailWindow+1, []*domain.GigPreview{gig}, false, ttl)
}

func (r *repo) ListPreviewWindow(ctx context.Context, userID string, window int) (*domain.ListPreviewGigsResult, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("get", "gig_preview", status, time.Since(started)) }()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, domain.ErrInvalidUsername
	}
	if window < 0 {
		window = 0
	}

	windowKey := GigPreviewWindowKey(userID, window)
	nextWindowKey := GigPreviewWindowKey(userID, window+1)
	const script = `
local ids = redis.call("ZREVRANGE", KEYS[1], 0, -1)
if #ids == 0 then
  return nil
end
local payloadKeys = {}
for i, id in ipairs(ids) do
  payloadKeys[i] = "gig:preview:" .. id
end
local payloads = redis.call("MGET", unpack(payloadKeys))
local nextCount = redis.call("ZCARD", KEYS[2]) or 0
local hasMore = 0
if nextCount > 0 then
  hasMore = 1
end
local out = {tostring(hasMore)}
for i, payload in ipairs(payloads) do
  if payload then
    table.insert(out, payload)
  end
end
return out`
	raw, err := r.rdb.Eval(ctx, script, []string{windowKey, nextWindowKey}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrGigNotFound
		}
		status = "error"
		return nil, err
	}
	if raw == nil {
		return nil, domain.ErrGigNotFound
	}
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		status = "error"
		return nil, fmt.Errorf("unexpected preview batch result type %T", raw)
	}
	cached := &domain.ListPreviewGigsResult{HasMore: stringValue(values[0]) == "1"}
	for _, item := range values[1:] {
		rawPayload, ok := item.(string)
		if !ok || strings.TrimSpace(rawPayload) == "" {
			continue
		}
		var gig domain.GigPreview
		if err := json.Unmarshal([]byte(rawPayload), &gig); err != nil {
			status = "error"
			return nil, err
		}
		cached.Gigs = append(cached.Gigs, &gig)
	}
	return cached, nil
}

func (r *repo) previewWindowIndexes(ctx context.Context, userID string) ([]int, error) {
	iter := r.rdb.Scan(ctx, 0, fmt.Sprintf("gig:preview:%s:window:*", strings.TrimSpace(userID)), 0).Iterator()
	indexes := make([]int, 0, 4)
	for iter.Next(ctx) {
		idx, ok := parsePreviewWindowIndex(iter.Val())
		if !ok {
			continue
		}
		indexes = append(indexes, idx)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sort.Ints(indexes)
	return indexes, nil
}

func parsePreviewWindowIndex(key string) (int, bool) {
	pos := strings.LastIndex(key, ":")
	if pos < 0 || pos+1 >= len(key) {
		return 0, false
	}
	window, err := strconv.Atoi(key[pos+1:])
	if err != nil {
		return 0, false
	}
	return window, true
}

func (r *repo) releasePreviewLease(ctx context.Context, leaseKey, token string) (bool, error) {
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`
	res, err := r.rdb.Eval(ctx, script, []string{leaseKey}, token).Int64()
	if err != nil {
		return false, WrapReleasePreviewLeaseError(leaseKey, err)
	}
	return res > 0, nil
}

func (r *repo) SetUserLookup(ctx context.Context, username, userID string, ttl time.Duration) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("set", "gig_lookup", status, time.Since(started)) }()

	username = strings.TrimSpace(username)
	userID = strings.TrimSpace(userID)
	if username == "" || userID == "" {
		return domain.ErrInvalidUsername
	}
	if err := r.rdb.Set(ctx, UserLookupKey(username), userID, ttl).Err(); err != nil {
		status = "error"
		return err
	}
	return nil
}

func (r *repo) GetUserLookup(ctx context.Context, username string) (string, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("get", "gig_lookup", status, time.Since(started)) }()

	username = strings.TrimSpace(username)
	if username == "" {
		return "", domain.ErrInvalidUsername
	}

	raw, err := r.rdb.Get(ctx, UserLookupKey(username)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrGigNotFound
		}
		status = "error"
		return "", err
	}

	return strings.TrimSpace(raw), nil
}

func (r *repo) SetPopularitySnapshot(ctx context.Context, snapshot *domain.GigPopularitySnapshot) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("set", "gig_popularity", status, time.Since(started)) }()

	if snapshot == nil || strings.TrimSpace(snapshot.GigID) == "" {
		return domain.ErrInvalidGigID
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		status = "error"
		return err
	}
	if err := r.rdb.Set(ctx, GigPopularityKey(snapshot.GigID), payload, 0).Err(); err != nil {
		status = "error"
		return err
	}
	return nil
}

func (r *repo) GetPopularitySnapshot(ctx context.Context, gigID string) (*domain.GigPopularitySnapshot, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("get", "gig_popularity", status, time.Since(started)) }()

	gigID = strings.TrimSpace(gigID)
	if gigID == "" {
		return nil, domain.ErrInvalidGigID
	}

	raw, err := r.rdb.Get(ctx, GigPopularityKey(gigID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrGigNotFound
		}
		status = "error"
		return nil, err
	}
	var snapshot domain.GigPopularitySnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		status = "error"
		return nil, err
	}
	return &snapshot, nil
}

func (r *repo) ListPopularitySnapshotsByGigIDs(ctx context.Context, gigIDs []string) (map[string]*domain.GigPopularitySnapshot, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("get", "gig_popularity", status, time.Since(started)) }()

	keys := make([]string, 0, len(gigIDs))
	lookup := make(map[string]string, len(gigIDs))
	for _, gigID := range gigIDs {
		gigID = strings.TrimSpace(gigID)
		if gigID == "" {
			continue
		}
		key := GigPopularityKey(gigID)
		keys = append(keys, key)
		lookup[key] = gigID
	}
	out := make(map[string]*domain.GigPopularitySnapshot, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	const script = `return redis.call("MGET", unpack(KEYS))`
	raw, err := r.rdb.Eval(ctx, script, keys).Result()
	if err != nil {
		status = "error"
		return nil, err
	}
	values, ok := raw.([]any)
	if !ok {
		status = "error"
		return nil, fmt.Errorf("unexpected popularity batch result type %T", raw)
	}
	for i, item := range values {
		if item == nil || i >= len(keys) {
			continue
		}
		var payload string
		switch v := item.(type) {
		case string:
			payload = v
		case []byte:
			payload = string(v)
		default:
			continue
		}
		var snapshot domain.GigPopularitySnapshot
		if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
			status = "error"
			return nil, err
		}
		if gigID := lookup[keys[i]]; gigID != "" {
			snapshot.GigID = gigID
			out[gigID] = &snapshot
		}
	}
	return out, nil
}

func (r *repo) ListPopularitySnapshots(ctx context.Context) ([]*domain.GigPopularitySnapshot, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveRedis("scan", "gig_popularity", status, time.Since(started)) }()

	iter := r.rdb.Scan(ctx, 0, "gig.popularity.*", 0).Iterator()
	out := make([]*domain.GigPopularitySnapshot, 0)
	for iter.Next(ctx) {
		raw, err := r.rdb.Get(ctx, iter.Val()).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			status = "error"
			return nil, err
		}
		var snapshot domain.GigPopularitySnapshot
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			status = "error"
			return nil, err
		}
		out = append(out, &snapshot)
	}
	if err := iter.Err(); err != nil {
		status = "error"
		return nil, err
	}
	return out, nil
}

// GigCacheKey builds the Redis key used for the gig read model.
// GigCacheKey builds the Redis key used for the gig read model.
func GigCacheKey(gigID string) string {
	return fmt.Sprintf("gig:%s", gigID)
}

// GigPreviewWindowKey builds the Redis key used for one cached freelancer
// preview window.
func GigPreviewWindowKey(userID string, window int) string {
	return fmt.Sprintf("gig:preview:%s:window:%d", strings.TrimSpace(userID), window)
}

// GigPreviewPayloadKey builds the Redis key used for one preview gig payload.
func GigPreviewPayloadKey(gigID string) string {
	return fmt.Sprintf("gig:preview:%s", strings.TrimSpace(gigID))
}

// GigPreviewLeaseKey builds the Redis key used to serialize preview window
// mutations for one seller.
func GigPreviewLeaseKey(userID string) string {
	return fmt.Sprintf("gig:preview:%s:lease", strings.TrimSpace(userID))
}

// UserLookupKey builds the Redis key used to warm username-to-userID lookups.
func UserLookupKey(username string) string {
	return fmt.Sprintf("user:%s", strings.TrimSpace(username))
}

// GigPopularityKey builds the Redis key used to store one popularity snapshot.
func GigPopularityKey(gigID string) string {
	return fmt.Sprintf("gig.popularity.%s", strings.TrimSpace(gigID))
}

func gigPopularityScore(gig *domain.GigPreview) int64 {
	if gig == nil {
		return 0
	}
	return gig.PopularityScore
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}
