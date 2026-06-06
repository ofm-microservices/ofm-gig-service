package grpc

import "errors"

var (
	// ErrEmptyReviewServiceAddr reports that the review-service address was not provided.
	ErrEmptyReviewServiceAddr = errors.New("review service address is empty")
	// ErrNilLogger reports that the logger dependency was not provided.
	ErrNilLogger = errors.New("logger is nil")
)
