package repository

import (
	"errors"
	"fmt"
)

var (
	ErrNilYugaByteDB        = errors.New("yugabyte db is nil")
	ErrNilDBErrorTranslator = errors.New("db error translator is nil")
	ErrNilLogger            = errors.New("logger is nil")
)

// WrapCreateGigError annotates gig insert failures.
func WrapCreateGigError(err error) error {
	return fmt.Errorf("create gig: %w", err)
}

// WrapFindGigError annotates gig lookup failures.
func WrapFindGigError(err error) error {
	return fmt.Errorf("find gig: %w", err)
}

// WrapPublishGigError annotates gig publication failures.
func WrapPublishGigError(err error) error {
	return fmt.Errorf("publish gig: %w", err)
}
