package grpc

import "errors"

var (
	// ErrNilLogger reports that the gig-service logger dependency is missing.
	ErrNilLogger = errors.New("logger is nil")
	// ErrEmptyOrderServiceAddr reports that the upstream order-service address
	// was not configured.
	ErrEmptyOrderServiceAddr = errors.New("order service address is empty")
)
