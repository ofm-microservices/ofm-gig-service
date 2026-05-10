package grpc

import "errors"

var (
	ErrNilGigService = errors.New("gig service is nil")
	ErrNilLogger     = errors.New("logger is nil")
)
