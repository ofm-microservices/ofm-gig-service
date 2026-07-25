package service

import (
	"context"
	"errors"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"sort"
	"strings"
	"time"
)

const (
	gigListSortCreatedAt    = "created_at"
	gigListSortUpdatedAt    = "updated_at"
	gigListSortPublishedAt  = "published_at"
	gigListSortRatingAvg    = "rating_avg"
	gigListSortTotalReviews = "total_reviews"
	gigListSortOrderCount   = "order_count"

	gigListOrderAsc  = "asc"
	gigListOrderDesc = "desc"
)

// GetMyGigs returns the authenticated owner's gig list projected from gig
// previews and paginated in application memory.
func (s *gigService) GetMyGigs(ctx context.Context, query domain.ListMyGigsQuery) (*domain.ListMyGigsResult, error) {
	ownerID := strings.TrimSpace(query.UserID)
	if ownerID == "" {
		return nil, domain.ErrInvalidFreelancerID
	}
	status := normalizeGigListStatus(query.Status)
	if status == gigListStatusInvalid {
		return nil, domain.ErrInvalidGigListStatus
	}
	sortKey := normalizeGigListSort(query.Sort)
	if sortKey == gigListSortInvalid {
		return nil, domain.ErrInvalidGigListSort
	}
	order := normalizeGigListOrder(query.Order)
	if order == gigListOrderInvalid {
		return nil, domain.ErrInvalidGigListOrder
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
		return nil, domain.ErrInvalidGigListLimit
	}

	cached, err := s.readRepo.ListOwnerPreviewGigs(ctx, ownerID)
	if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
		return nil, err
	}
	if len(cached) == 0 {
		return s.buildMyGigsFromSource(ctx, ownerID, status, sortKey, order, page, limit)
	}

	items := filterOwnerGigs(cached, status)
	sortOwnerGigs(items, sortKey, order)
	return paginateOwnerGigs(items, page, limit), nil
}

func (s *gigService) buildMyGigsFromSource(ctx context.Context, ownerID, status, sortKey, order string, page, limit int) (*domain.ListMyGigsResult, error) {
	gigs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	owned := make([]*domain.Gig, 0, len(gigs))
	for _, gig := range gigs {
		if gig == nil || strings.TrimSpace(gig.FreelancerID) != ownerID {
			continue
		}
		full, loadErr := s.repo.GetByID(ctx, gig.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		owned = append(owned, full)
	}
	if len(owned) == 0 {
		return &domain.ListMyGigsResult{Page: page, Limit: limit}, nil
	}

	if err := s.RebuildPopularitySnapshots(ctx, owned); err != nil {
		s.log.Warn("gig popularity snapshot rebuild failed",
			logging.Operation("gig.my_gigs.popularity_rebuild"),
			logging.Err(err),
		)
	}

	previewed := make([]*domain.GigPreview, 0, len(owned))
	for _, gig := range owned {
		if err := s.UpsertPreviewGig(ctx, gig); err != nil {
			s.log.Warn("owner preview cache upsert failed",
				logging.Operation("gig.my_gigs.cache_upsert"),
				logging.String("gig_id", gig.ID),
				logging.String("user_id", ownerID),
				logging.Err(err),
			)
		}
		preview, getErr := s.readRepo.GetPreviewByID(ctx, gig.ID)
		if getErr != nil {
			return nil, getErr
		}
		previewed = append(previewed, preview)
	}

	items := filterOwnerGigs(previewed, status)
	sortOwnerGigs(items, sortKey, order)
	return paginateOwnerGigs(items, page, limit), nil
}

func filterOwnerGigs(items []*domain.GigPreview, status string) []*domain.GigPreview {
	if status == "" {
		return append([]*domain.GigPreview(nil), items...)
	}
	out := make([]*domain.GigPreview, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Status), status) {
			out = append(out, item)
		}
	}
	return out
}

func sortOwnerGigs(items []*domain.GigPreview, sortKey, order string) {
	ascending := order == gigListOrderAsc
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return right != nil
		}
		cmp := compareOwnerGig(left, right, sortKey)
		if cmp == 0 {
			return strings.TrimSpace(left.ID) > strings.TrimSpace(right.ID)
		}
		if ascending {
			return cmp < 0
		}
		return cmp > 0
	})
}

func compareOwnerGig(left, right *domain.GigPreview, sortKey string) int {
	switch sortKey {
	case gigListSortCreatedAt:
		return compareTime(left.CreatedAt, right.CreatedAt)
	case gigListSortUpdatedAt:
		return compareTime(left.UpdatedAt, right.UpdatedAt)
	case gigListSortPublishedAt:
		return compareOptionalTime(left.PublishedAt, right.PublishedAt)
	case gigListSortRatingAvg:
		return compareFloat(left.RatingAvg, right.RatingAvg)
	case gigListSortTotalReviews:
		return compareInt64(left.TotalReviews, right.TotalReviews)
	case gigListSortOrderCount:
		return compareInt64(left.OrderCount, right.OrderCount)
	default:
		return compareTime(left.UpdatedAt, right.UpdatedAt)
	}
}

func compareTime(left, right time.Time) int {
	switch {
	case left.Before(right):
		return -1
	case right.Before(left):
		return 1
	default:
		return 0
	}
}

func compareOptionalTime(left, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return compareTime(*left, *right)
	}
}

func compareFloat(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func paginateOwnerGigs(items []*domain.GigPreview, page, limit int) *domain.ListMyGigsResult {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	start := (page - 1) * limit
	if start >= len(items) {
		totalPages := 0
		if len(items) > 0 {
			totalPages = (len(items) + limit - 1) / limit
		}
		return &domain.ListMyGigsResult{Gigs: []*domain.GigPreview{}, Page: page, Limit: limit, TotalPages: totalPages}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	totalPages := 0
	if len(items) > 0 {
		totalPages = (len(items) + limit - 1) / limit
	}
	out := &domain.ListMyGigsResult{
		Gigs:       items[start:end],
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}
	out.HasMore = end < len(items)
	return out
}

const (
	gigListStatusInvalid = "__invalid_status__"
	gigListSortInvalid   = "__invalid_sort__"
	gigListOrderInvalid  = "__invalid_order__"
)

func normalizeGigListStatus(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "", domain.StatusDraft, domain.StatusPublished, domain.StatusPaused, domain.StatusArchived:
		return raw
	default:
		return gigListStatusInvalid
	}
}

func normalizeGigListSort(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "", gigListSortCreatedAt, gigListSortUpdatedAt, gigListSortPublishedAt, gigListSortRatingAvg, gigListSortTotalReviews, gigListSortOrderCount:
		if raw == "" {
			return gigListSortUpdatedAt
		}
		return raw
	default:
		return gigListSortInvalid
	}
}

func normalizeGigListOrder(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "", gigListOrderDesc:
		return gigListOrderDesc
	case gigListOrderAsc:
		return gigListOrderAsc
	default:
		return gigListOrderInvalid
	}
}
