package grpc

import (
	"context"

	app "gig-service/internal/application"
	reviewv1 "github.com/ofm-microservices/ofm-common/proto/review/v1"
)

// ReviewService exposes the review-service rating lookup used by gig-service
// to backfill gig preview aggregates.
type ReviewService interface {
	GetGigRatingSummary(ctx context.Context, gigID string) (*app.ReviewSummary, error)
	Close() error
}

// ReviewMapper translates between gig-service inputs and the shared review
// gRPC contract.
type ReviewMapper interface {
	ToGetGigRatingSummaryRequest(gigID string) *reviewv1.GetGigRatingSummaryRequest
	ToGetGigRatingSummaryResponse(res *reviewv1.RatingSummary) *app.ReviewSummary
}
