package grpc

import "errors"

var (
	// ErrNilLogger reports that the logger dependency was not provided.
	ErrNilLogger = errors.New("logger is nil")
	// ErrEmptyPaymentServiceAddr reports that the payment-service address was empty.
	ErrEmptyPaymentServiceAddr = errors.New("payment service address is empty")
)
