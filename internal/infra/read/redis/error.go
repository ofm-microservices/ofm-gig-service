package repository

import (
	"errors"
	"fmt"
)

var (
	ErrNilRedisClient = errors.New("redis client is nil")
	ErrNilGig         = errors.New("gig is nil")
	ErrNilLogger      = errors.New("logger is nil")
	// ErrPreviewLeaseBusy reports that another preview append is currently
	// holding the seller lease.
	ErrPreviewLeaseBusy = errors.New("preview lease is busy")
)

// WrapMarshalGigCacheError annotates cache serialization failures.
func WrapMarshalGigCacheError(err error) error {
	return fmt.Errorf("marshal gig cache: %w", err)
}

// WrapSetGigCacheError annotates Redis upsert failures for the gig cache.
func WrapSetGigCacheError(key string, err error) error {
	return fmt.Errorf("set gig cache by key %q: %w", key, err)
}

// WrapGetGigCacheError annotates Redis fetch failures for the gig cache.
func WrapGetGigCacheError(key string, err error) error {
	return fmt.Errorf("get gig cache by key %q: %w", key, err)
}

// WrapUnmarshalGigCacheError annotates cache deserialization failures.
func WrapUnmarshalGigCacheError(err error) error {
	return fmt.Errorf("unmarshal gig cache: %w", err)
}

// WrapDeleteGigCacheError annotates Redis delete failures for the gig cache.
func WrapDeleteGigCacheError(key string, err error) error {
	return fmt.Errorf("delete gig cache by key %q: %w", key, err)
}

// WrapPreviewLeaseError annotates preview lease acquisition failures.
func WrapPreviewLeaseError(key string, err error) error {
	return fmt.Errorf("acquire preview lease by key %q: %w", key, err)
}

// WrapReleasePreviewLeaseError annotates preview lease release failures.
func WrapReleasePreviewLeaseError(key string, err error) error {
	return fmt.Errorf("release preview lease by key %q: %w", key, err)
}
