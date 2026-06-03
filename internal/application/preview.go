package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type previewCursorState struct {
	Window    int    `json:"window"`
	Page      int    `json:"page"`
	LastScore int64  `json:"last_score"`
	LastID    string `json:"last_id"`
}

func (s *gigService) GetPreviewGigsByFreelancerUsername(ctx context.Context, query domain.ListPreviewGigsQuery) (*domain.ListPreviewGigsResult, error) {
	username := strings.TrimSpace(query.SellerUsername)
	if username == "" {
		return nil, domain.ErrInvalidUsername
	}

	state, err := s.decodePreviewCursor(query.Cursor)
	if err != nil {
		return nil, err
	}

	ownerID, _ := s.readRepo.GetUserLookup(ctx, username)
	if ownerID == "" {
		ownerID, err = s.resolvePreviewOwnerID(ctx, username)
		if err != nil {
			return nil, err
		}
		if ownerID != "" {
			_ = s.readRepo.SetUserLookup(ctx, username, ownerID, 0)
		}
	}
	if ownerID == "" {
		return &domain.ListPreviewGigsResult{}, nil
	}

	current, err := s.readRepo.ListPreviewWindow(ctx, ownerID, state.Window)
	if err == nil && current != nil && len(current.Gigs) > 0 {
		return s.pagePreviewWindow(current, state)
	}
	if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
		return nil, err
	}

	gigs, err := s.repo.ListPublishedBySellerUsername(ctx, domain.ListPreviewGigsQuery{SellerUsername: username, Limit: 0})
	if err != nil {
		return nil, err
	}
	if err := s.RebuildPopularitySnapshots(ctx, gigs); err != nil {
		return nil, err
	}
	popularity, err := s.readRepo.ListPopularitySnapshotsByGigIDs(ctx, gigIDs(gigs))
	if err != nil {
		return nil, err
	}
	current, err = s.buildPreviewWindows(ctx, ownerID, username, gigs, popularity)
	if err != nil {
		return nil, err
	}

	return s.pagePreviewWindow(current, state)
}

func (s *gigService) pagePreviewWindow(current *domain.ListPreviewGigsResult, state previewCursorState) (*domain.ListPreviewGigsResult, error) {
	if current == nil || len(current.Gigs) == 0 {
		return &domain.ListPreviewGigsResult{}, nil
	}

	pageInWindow := ((state.Page - 1) % s.previewPagesWindow) + 1
	pageStart := (pageInWindow - 1) * s.previewPageSize
	pageEnd := pageStart + s.previewPageSize
	if pageStart >= len(current.Gigs) {
		return &domain.ListPreviewGigsResult{}, nil
	}
	if pageEnd > len(current.Gigs) {
		pageEnd = len(current.Gigs)
	}
	page := current.Gigs[pageStart:pageEnd]
	result := &domain.ListPreviewGigsResult{
		Gigs:    page,
		HasMore: current.HasMore || pageEnd < len(current.Gigs),
	}
	if result.HasMore && len(page) > 0 {
		last := page[len(page)-1]
		nextWindow := state.Window
		nextPage := state.Page + 1
		if pageInWindow >= s.previewPagesWindow {
			nextWindow++
			nextPage = 1
		}
		result.Cursor = s.encodePreviewCursor(previewCursorState{
			Window:    nextWindow,
			Page:      nextPage,
			LastScore: previewScoreOf(last),
			LastID:    last.ID,
		})
	}
	return result, nil
}

func (s *gigService) resolvePreviewOwnerID(ctx context.Context, username string) (string, error) {
	gigs, err := s.repo.ListPublishedBySellerUsername(ctx, domain.ListPreviewGigsQuery{SellerUsername: username, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(gigs) == 0 {
		return "", nil
	}
	return gigs[0].FreelancerID, nil
}

func (s *gigService) buildPreviewWindows(ctx context.Context, ownerID, username string, gigs []*domain.Gig, popularity map[string]*domain.GigPopularitySnapshot) (*domain.ListPreviewGigsResult, error) {
	previewed := make([]*domain.GigPreview, 0, len(gigs))
	for _, gig := range gigs {
		projected, err := s.Project(ctx, gig)
		if err != nil {
			return nil, err
		}
		if projected == nil {
			continue
		}
		previewed = append(previewed, s.toPreview(projected, popularity[projected.ID]))
	}
	sort.SliceStable(previewed, func(i, j int) bool {
		si := previewScoreOf(previewed[i])
		sj := previewScoreOf(previewed[j])
		if si == sj {
			return previewed[i].ID > previewed[j].ID
		}
		return si > sj
	})

	for window := 0; window*s.previewWindowSize < len(previewed); window++ {
		start := window * s.previewWindowSize
		end := start + s.previewWindowSize
		if end > len(previewed) {
			end = len(previewed)
		}
		hasMore := end < len(previewed)
		if err := s.readRepo.UpsertPreviewWindow(ctx, ownerID, window, previewed[start:end], hasMore, s.previewWindowTTL); err != nil {
			s.log.Error("upsert preview window failed",
				logging.Operation("gig.preview.window_upsert"),
				logging.String("username", username),
				logging.String("user_id", ownerID),
				logging.Int("window", window),
				logging.Err(err),
			)
		}
	}

	if len(previewed) == 0 {
		return &domain.ListPreviewGigsResult{}, nil
	}
	return &domain.ListPreviewGigsResult{
		Gigs:    previewed[:min(s.previewWindowSize, len(previewed))],
		HasMore: len(previewed) > s.previewWindowSize,
	}, nil
}

func (s *gigService) toPreview(gig *domain.Gig, snapshot *domain.GigPopularitySnapshot) *domain.GigPreview {
	minPrice := int64(0)
	for i, pkg := range gig.Packages {
		if i == 0 || pkg.PriceCents < minPrice {
			minPrice = pkg.PriceCents
		}
	}
	score := int64(0)
	if snapshot != nil {
		score = snapshot.Score
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
		CreatedAt:         gig.CreatedAt,
	}
}

func gigIDs(gigs []*domain.Gig) []string {
	out := make([]string, 0, len(gigs))
	for _, gig := range gigs {
		if gig == nil || strings.TrimSpace(gig.ID) == "" {
			continue
		}
		out = append(out, gig.ID)
	}
	return out
}

func previewScoreOf(gig *domain.GigPreview) int64 {
	if gig == nil {
		return 0
	}
	return gig.PopularityScore
}

func (s *gigService) decodePreviewCursor(raw string) (previewCursorState, error) {
	if strings.TrimSpace(raw) == "" {
		return previewCursorState{Window: 0, Page: 1}, nil
	}
	var state previewCursorState
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return previewCursorState{}, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return previewCursorState{}, err
	}
	if state.Page <= 0 {
		state.Page = 1
	}
	return state, nil
}

func (s *gigService) encodePreviewCursor(state previewCursorState) string {
	data, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
