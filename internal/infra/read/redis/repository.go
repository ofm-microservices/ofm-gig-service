package repository

import (
	"context"
	"errors"
	"fmt"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
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

// GigCacheKey builds the Redis key used for the gig read model.
func GigCacheKey(gigID string) string {
	return fmt.Sprintf("gig:%s", gigID)
}
