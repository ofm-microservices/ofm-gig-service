package grpc

import "errors"

var (
	// ErrEmptyFileServiceAddress reports a missing upstream file-service
	// address.
	ErrEmptyFileServiceAddress = errors.New("file service address is empty")
	// ErrNilLogger reports a missing logger dependency.
	ErrNilLogger = errors.New("logger is nil")
)
