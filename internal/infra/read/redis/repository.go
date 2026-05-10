package repository

import (
	"context"
	"fmt"
	app "gig-service/internal/application"
	"gig-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

type repo struct {
	rdb  *redis.Client
	mapr app.GigEventMapper
}

// New constructs the Redis-backed gig read-model repository.
func New(rdb *redis.Client, mapr app.GigEventMapper) (domain.GigReadRepository, error) {
	if rdb == nil {
		return nil, ErrNilRedisClient
	}
	if mapr == nil {
		return nil, ErrNilGig
	}

	return &repo{rdb: rdb, mapr: mapr}, nil
}

// Upsert stores the gig read model in Redis.
func (r *repo) Upsert(ctx context.Context, gig *domain.Gig) error {
	if gig == nil {
		return ErrNilGig
	}

	payload, err := r.mapr.ToPublishedPayload(gig)
	if err != nil {
		return WrapMarshalGigCacheError(err)
	}

	key := GigCacheKey(gig.ID)
	if err := r.rdb.Set(ctx, key, payload, 0).Err(); err != nil {
		return WrapSetGigCacheError(key, err)
	}

	return nil
}

// DeleteByID removes the gig read model from Redis.
func (r *repo) DeleteByID(ctx context.Context, gigID string) error {
	key := GigCacheKey(gigID)
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return WrapDeleteGigCacheError(key, err)
	}

	return nil
}

// GigCacheKey builds the Redis key used for the gig read model.
func GigCacheKey(gigID string) string {
	return fmt.Sprintf("gig:%s", gigID)
}
