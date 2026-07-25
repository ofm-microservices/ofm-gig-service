package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"gig-service/internal/domain"
)

func (s *gigService) GetPreviewGigsByFreelancerUsername(ctx context.Context, query domain.ListPreviewGigsQuery) (*domain.ListPreviewGigsResult, error) {
	username := strings.TrimSpace(query.SellerUsername)
	if username == "" {
		return nil, domain.ErrInvalidUsername
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = s.previewPageSize
	}
	if limit <= 0 {
		limit = 10
	}

	gigs, err := s.repo.ListPublishedBySellerUsername(ctx, domain.ListPreviewGigsQuery{SellerUsername: username, Limit: 0})
	if err != nil {
		return nil, err
	}
	if len(gigs) == 0 {
		return &domain.ListPreviewGigsResult{Gigs: []*domain.GigPreview{}, Page: page, Limit: limit, TotalPages: 0}, nil
	}

	if err := s.RebuildPopularitySnapshots(ctx, gigs); err != nil {
		// best effort; stale popularity should not block the public list
	}
	popularity, err := s.readRepo.ListPopularitySnapshotsByGigIDs(ctx, gigIDs(gigs))
	if err != nil {
		popularity = map[string]*domain.GigPopularitySnapshot{}
	}

	items := make([]*domain.GigPreview, 0, len(gigs))
	for _, gig := range gigs {
		cached, cacheErr := s.readRepo.GetPreviewByID(ctx, gig.ID)
		if cacheErr == nil && cached != nil {
			items = append(items, cached)
			continue
		}
		if err := s.UpsertPreviewGig(ctx, gig); err != nil {
			return nil, err
		}
		cached, cacheErr = s.readRepo.GetPreviewByID(ctx, gig.ID)
		if cacheErr == nil && cached != nil {
			items = append(items, cached)
			continue
		}
		projected, err := s.Project(ctx, gig)
		if err != nil {
			return nil, err
		}
		if projected == nil {
			continue
		}
		items = append(items, s.toPreview(projected, popularity[projected.ID], nil, nil))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if previewScoreOf(items[i]) == previewScoreOf(items[j]) {
			return items[i].ID > items[j].ID
		}
		return previewScoreOf(items[i]) > previewScoreOf(items[j])
	})

	if len(items) > 0 {
		userID := strings.TrimSpace(items[0].FreelancerID)
		if userID != "" {
			if err := s.readRepo.UpsertPreviewWindow(ctx, userID, 0, items, false, 0); err != nil {
				// best effort; a cache write failure should not block the page.
			}
		}
	}

	return paginatePreviewItems(items, page, limit), nil
}

func gigIDs(gigs []*domain.Gig) []string {
	ids := make([]string, 0, len(gigs))
	for _, gig := range gigs {
		if gig == nil {
			continue
		}
		ids = append(ids, gig.ID)
	}
	return ids
}

func previewScoreOf(gig *domain.GigPreview) int64 {
	if gig == nil {
		return 0
	}
	return gig.PopularityScore
}

func paginatePreviewItems(items []*domain.GigPreview, page, limit int) *domain.ListPreviewGigsResult {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	totalPages := 0
	if len(items) > 0 {
		totalPages = (len(items) + limit - 1) / limit
	}
	start := (page - 1) * limit
	if start >= len(items) {
		return &domain.ListPreviewGigsResult{
			Gigs:       []*domain.GigPreview{},
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
			HasMore:    false,
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return &domain.ListPreviewGigsResult{
		Gigs:       items[start:end],
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    end < len(items),
	}
}

func (s *gigService) toPreview(gig *domain.Gig, snapshot *domain.GigPopularitySnapshot, rating *ReviewSummary, orderCount *OrderCountResult) *domain.GigPreview {
	minPrice := int64(0)
	for i, pkg := range gig.Packages {
		if i == 0 {
			minPrice = pkg.PriceCents
			continue
		}
		if pkg.PriceCents < minPrice {
			minPrice = pkg.PriceCents
		}
	}
	score := int64(0)
	popularityOrders := int64(0)
	if snapshot != nil {
		score = snapshot.Score
		popularityOrders = snapshot.CompletedOrdersLast30d
	}
	var publishedAt *time.Time
	if gig.PublishedAt != nil {
		publishedAt = gig.PublishedAt
	}
	ratingAvg := 0.0
	totalReviews := int64(0)
	if rating != nil {
		ratingAvg = rating.RatingAvg
		totalReviews = rating.TotalReviews
	}
	orderTotal := popularityOrders
	if orderCount != nil {
		orderTotal = orderCount.OrderCount
	}
	return &domain.GigPreview{
		ID:                gig.ID,
		FreelancerID:      gig.FreelancerID,
		SellerUsername:    gig.SellerUsername,
		Slug:              gig.Slug,
		Title:             gig.Title,
		ShortInfo:         gig.ShortInfo,
		MinimumPriceCents: minPrice,
		PictureURL:        gig.PictureURL,
		PopularityScore:   score,
		Status:            gig.Status,
		PublishedAt:       publishedAt,
		UpdatedAt:         gig.UpdatedAt,
		RatingAvg:         ratingAvg,
		TotalReviews:      totalReviews,
		OrderCount:        orderTotal,
		CreatedAt:         gig.CreatedAt,
	}
}
