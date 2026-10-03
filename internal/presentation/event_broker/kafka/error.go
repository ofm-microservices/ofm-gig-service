package kafka

import "errors"

// Transport adapter construction and payload errors.
var (
	ErrInvalidSubscriberDependency = errors.New("kafka subscriber dependency is invalid")
	ErrInvalidGigPayload           = errors.New("gig event payload is invalid")
)
