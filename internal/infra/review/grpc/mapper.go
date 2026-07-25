package grpc

import (
	app "gig-service/internal/application"
	reviewv1 "github.com/ofm-microservices/ofm-common/proto/review/v1"
)

type reviewMapper struct{}

func newReviewMapper() ReviewMapper { return &reviewMapper{} }

func (m *reviewMapper) ToGetGigRatingSummaryRequest(gigID string) *reviewv1.GetGigRatingSummaryRequest {
	return &reviewv1.GetGigRatingSummaryRequest{GigId: gigID}
}

func (m *reviewMapper) ToGetGigRatingSummaryResponse(res *reviewv1.RatingSummary) *app.ReviewSummary {
	if res == nil {
		return nil
	}
	return &app.ReviewSummary{
		RatingAvg:    res.GetRatingAvg(),
		TotalReviews: res.GetTotalReviews(),
	}
}
