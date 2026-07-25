package service

import "errors"

var (
	// ErrNilGigRepository reports that the gig repository dependency was not provided.
	ErrNilGigRepository = errors.New("gig repository is nil")
	// ErrNilGigReadRepository reports that the gig read repository dependency was not provided.
	ErrNilGigReadRepository = errors.New("gig read repository is nil")
	// ErrNilFileService reports that the file-service dependency was not provided.
	ErrNilFileService = errors.New("file service is nil")
	// ErrNilConnectStatusChecker reports that the payment-service Connect lookup dependency was not provided.
	ErrNilConnectStatusChecker = errors.New("connect status checker is nil")
	// ErrNilReviewClient reports that the review-service rating lookup dependency was not provided.
	ErrNilReviewClient = errors.New("review client is nil")
	// ErrNilOrderCountClient reports that the order-service order count lookup dependency was not provided.
	ErrNilOrderCountClient = errors.New("order count client is nil")
	// ErrNilEventBroker reports that the event broker dependency was not provided.
	ErrNilEventBroker = errors.New("event broker is nil")
	// ErrNilPopularitySource reports that the popularity analytics dependency was not provided.
	ErrNilPopularitySource = errors.New("popularity source is nil")
	// ErrNilSlugger reports that the slugger dependency was not provided.
	ErrNilSlugger = errors.New("slugger is nil")
	// ErrNilLogger reports that the logger dependency was not provided.
	ErrNilLogger = errors.New("logger is nil")
	// ErrInvalidPreviewConfig reports that the preview pagination configuration is invalid.
	ErrInvalidPreviewConfig = errors.New("preview pagination config is invalid")
)
