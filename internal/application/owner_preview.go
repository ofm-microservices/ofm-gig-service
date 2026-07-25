package service

import (
	"context"
	"errors"
	"gig-service/internal/domain"
	"strings"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

// UpsertPreviewGig refreshes the owner preview projection for one gig.
func (s *gigService) UpsertPreviewGig(ctx context.Context, gig *domain.Gig) error {
	if gig == nil {
		return domain.ErrGigNotFound
	}

	projected, err := s.Project(ctx, gig)
	if err != nil {
		return err
	}
	if projected == nil {
		return domain.ErrGigNotFound
	}

	popularity, err := s.readRepo.GetPopularitySnapshot(ctx, projected.ID)
	if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
		return err
	}

	var rating *ReviewSummary
	orderTotal := int64(0)
	if cached, cacheErr := s.readRepo.GetPreviewByID(ctx, projected.ID); cacheErr == nil && cached != nil {
		rating = &ReviewSummary{
			RatingAvg:    cached.RatingAvg,
			TotalReviews: cached.TotalReviews,
		}
		orderTotal = cached.OrderCount
	} else {
		summary, sumErr := s.review.GetGigRatingSummary(ctx, projected.ID)
		if sumErr == nil && summary != nil {
			rating = summary
		} else if sumErr != nil {
			s.log.Warn("gig rating summary backfill failed",
				logging.Operation("gig.preview.rating_backfill"),
				logging.String("gig_id", projected.ID),
				logging.Err(sumErr),
			)
		}
		count, countErr := s.orderCount.GetOrderCountByGigID(ctx, projected.ID)
		if countErr == nil && count != nil {
			orderTotal = count.OrderCount
		} else if countErr != nil {
			s.log.Warn("gig order count backfill failed",
				logging.Operation("gig.preview.order_count_backfill"),
				logging.String("gig_id", projected.ID),
				logging.Err(countErr),
			)
		}
	}

	preview := s.toPreview(projected, popularity, rating, &OrderCountResult{OrderCount: orderTotal})
	return s.readRepo.UpsertOwnerPreview(ctx, strings.TrimSpace(projected.FreelancerID), preview)
}

// RefreshPreviewRating refreshes the cached gig preview rating fields after a
// review aggregate update.
func (s *gigService) RefreshPreviewRating(ctx context.Context, gigID string) error {
	gigID = strings.TrimSpace(gigID)
	if gigID == "" {
		return domain.ErrInvalidGigID
	}

	preview, err := s.readRepo.GetPreviewByID(ctx, gigID)
	if err != nil {
		gig, loadErr := s.repo.GetByID(ctx, gigID)
		if loadErr != nil {
			return loadErr
		}
		return s.UpsertPreviewGig(ctx, gig)
	}
	summary, err := s.review.GetGigRatingSummary(ctx, gigID)
	if err != nil {
		return err
	}
	preview.RatingAvg = summary.RatingAvg
	preview.TotalReviews = summary.TotalReviews
	return s.readRepo.UpsertOwnerPreview(ctx, preview.FreelancerID, preview)
}

// RefreshPreviewOrderCount refreshes the cached gig preview order count after
// an order lifecycle event updates the canonical count.
func (s *gigService) RefreshPreviewOrderCount(ctx context.Context, gigID string) error {
	gigID = strings.TrimSpace(gigID)
	if gigID == "" {
		return domain.ErrInvalidGigID
	}

	preview, err := s.readRepo.GetPreviewByID(ctx, gigID)
	if err != nil {
		gig, loadErr := s.repo.GetByID(ctx, gigID)
		if loadErr != nil {
			return loadErr
		}
		return s.UpsertPreviewGig(ctx, gig)
	}

	return s.incrementPreviewOrderCount(ctx, preview)
}

func (s *gigService) incrementPreviewOrderCount(ctx context.Context, preview *domain.GigPreview) error {
	if preview == nil {
		return domain.ErrGigNotFound
	}
	preview.OrderCount++
	return s.readRepo.UpsertOwnerPreview(ctx, preview.FreelancerID, preview)
}
