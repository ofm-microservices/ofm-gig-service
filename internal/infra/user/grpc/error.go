package grpc

import "errors"

var (
	ErrEmptyAddress = errors.New("empty address")
	ErrNilLogger    = errors.New("nil logger")
)
