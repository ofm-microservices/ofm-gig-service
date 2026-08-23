package service

import (
	"testing"
	"time"

	"gig-service/internal/domain"
)

func TestGigEventMapperReadModelRoundTripPreservesCanonicalFields(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	original := &domain.Gig{
		ID: "gig-1", FreelancerID: "user-1", SellerUsername: "seller", Slug: "slug-1",
		Title: "Title", Description: "Description", Status: domain.StatusPublished,
		BasicInfoCompleted: true, PackagesCompleted: true, RequirementsCompleted: true, MediaCompleted: true,
		PublishedAt: &now, CreatedAt: now, UpdatedAt: now,
		Questions: []domain.GigQuestion{{ID: "q-1", GigID: "gig-1", Content: "Need what?", SortOrder: 1}},
	}

	mapper := &gigEventMapper{}
	payload, err := mapper.ToReadModelPayload(original)
	if err != nil {
		t.Fatalf("marshal read model: %v", err)
	}
	actual, err := mapper.FromReadModelPayload(payload)
	if err != nil {
		t.Fatalf("unmarshal read model: %v", err)
	}

	if actual.Status != original.Status || !actual.BasicInfoCompleted || !actual.PackagesCompleted || !actual.RequirementsCompleted || !actual.MediaCompleted {
		t.Fatalf("canonical status/completion fields were not preserved: %#v", actual)
	}
	if len(actual.Questions) != 1 || actual.Questions[0] != original.Questions[0] {
		t.Fatalf("questions were not preserved: %#v", actual.Questions)
	}
	if actual.Packages == nil || actual.Media == nil {
		t.Fatal("empty read-model collections must be normalized to non-nil slices")
	}
}
