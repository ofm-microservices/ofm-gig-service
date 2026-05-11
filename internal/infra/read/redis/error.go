package repository

import (
	"errors"
	"fmt"
)

var (
	ErrNilRedisClient = errors.New("redis client is nil")
	ErrNilGig         = errors.New("gig is nil")
	ErrNilLogger      = errors.New("logger is nil")
)

// WrapMarshalGigCacheError annotates cache serialization failures.
func WrapMarshalGigCacheError(err error) error {
	return fmt.Errorf("marshal gig cache: %w", err)
}

// WrapSetGigCacheError annotates Redis upsert failures for the gig cache.
func WrapSetGigCacheError(key string, err error) error {
	return fmt.Errorf("set gig cache by key %q: %w", key, err)
}

// WrapDeleteGigCacheError annotates Redis delete failures for the gig cache.
func WrapDeleteGigCacheError(key string, err error) error {
	return fmt.Errorf("delete gig cache by key %q: %w", key, err)
}
