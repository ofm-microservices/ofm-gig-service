package service

import (
	"encoding/json"
	"gig-service/internal/domain"
	"time"
)

const gigPublishedSubject = "gig.published"

// GigEventMapper builds outbound event payloads for publication.
type GigEventMapper interface {
	FromPublishedPayload(payload []byte) (*domain.Gig, error)
	ToPublishedPayload(gig *domain.Gig) ([]byte, error)
}

type gigEventMapper struct{}

// NewGigEventMapper constructs the shared gig event mapper used for publish
// and projection payloads.
func NewGigEventMapper() GigEventMapper {
	return &gigEventMapper{}
}

func (m *gigEventMapper) ToPublishedPayload(gig *domain.Gig) ([]byte, error) {
	payload := GigPublishedEvent{
		GigID:                 gig.ID,
		FreelancerID:          gig.FreelancerID,
		Slug:                  gig.Slug,
		Title:                 gig.Title,
		Description:           gig.Description,
		CategoryID:            gig.CategoryID,
		Currency:              gig.Currency,
		Status:                gig.Status,
		BasicInfoCompleted:    gig.BasicInfoCompleted,
		PackagesCompleted:     gig.PackagesCompleted,
		RequirementsCompleted: gig.RequirementsCompleted,
		MediaCompleted:        gig.MediaCompleted,
		PictureFileID:         gig.PictureFileID,
		PublishedAt:           gig.PublishedAt,
		CreatedAt:             gig.CreatedAt,
		UpdatedAt:             gig.UpdatedAt,
	}

	if len(gig.Packages) > 0 {
		payload.Packages = make([]GigPublishedPackage, 0, len(gig.Packages))
		for _, pkg := range gig.Packages {
			payload.Packages = append(payload.Packages, GigPublishedPackage{
				ID:           pkg.ID,
				GigID:        pkg.GigID,
				Tier:         pkg.Tier,
				Description:  pkg.Description,
				DeliveryDays: pkg.DeliveryDays,
				PriceCents:   pkg.PriceCents,
				SortOrder:    pkg.SortOrder,
			})
		}
	}

	if len(gig.Questions) > 0 {
		payload.Questions = make([]GigPublishedQuestion, 0, len(gig.Questions))
		for _, q := range gig.Questions {
			payload.Questions = append(payload.Questions, GigPublishedQuestion{
				ID:        q.ID,
				GigID:     q.GigID,
				Content:   q.Content,
				SortOrder: q.SortOrder,
			})
		}
	}

	if len(gig.Media) > 0 {
		payload.Media = make([]GigPublishedMedia, 0, len(gig.Media))
		for _, item := range gig.Media {
			payload.Media = append(payload.Media, GigPublishedMedia{
				ID:        item.FileID,
				GigID:     item.GigID,
				FileID:    item.FileID,
				SortOrder: item.SortOrder,
			})
		}
	}

	return json.Marshal(payload)
}

func (m *gigEventMapper) FromPublishedPayload(payload []byte) (*domain.Gig, error) {
	var event GigPublishedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	gig := &domain.Gig{
		ID:                    event.GigID,
		FreelancerID:          event.FreelancerID,
		Slug:                  event.Slug,
		Title:                 event.Title,
		Description:           event.Description,
		CategoryID:            event.CategoryID,
		Currency:              event.Currency,
		Status:                event.Status,
		BasicInfoCompleted:    event.BasicInfoCompleted,
		PackagesCompleted:     event.PackagesCompleted,
		RequirementsCompleted: event.RequirementsCompleted,
		MediaCompleted:        event.MediaCompleted,
		PictureFileID:         event.PictureFileID,
		PublishedAt:           event.PublishedAt,
		CreatedAt:             event.CreatedAt,
		UpdatedAt:             event.UpdatedAt,
	}

	if len(event.Packages) > 0 {
		gig.Packages = make([]domain.GigPackage, 0, len(event.Packages))
		for _, pkg := range event.Packages {
			gig.Packages = append(gig.Packages, domain.GigPackage{
				ID:           pkg.ID,
				GigID:        pkg.GigID,
				Tier:         pkg.Tier,
				Description:  pkg.Description,
				DeliveryDays: pkg.DeliveryDays,
				PriceCents:   pkg.PriceCents,
				SortOrder:    pkg.SortOrder,
			})
		}
	}

	if len(event.Questions) > 0 {
		gig.Questions = make([]domain.GigQuestion, 0, len(event.Questions))
		for _, q := range event.Questions {
			gig.Questions = append(gig.Questions, domain.GigQuestion{
				ID:        q.ID,
				GigID:     q.GigID,
				Content:   q.Content,
				SortOrder: q.SortOrder,
			})
		}
	}

	if len(event.Media) > 0 {
		gig.Media = make([]domain.GigMedia, 0, len(event.Media))
		for _, item := range event.Media {
			gig.Media = append(gig.Media, domain.GigMedia{
				GigID:     item.GigID,
				FileID:    item.FileID,
				SortOrder: item.SortOrder,
			})
		}
	}

	return gig, nil
}

// GigPublishedEvent is the JSON payload published to gig.published and later
// consumed by the gig projection subscriber.
type GigPublishedEvent struct {
	GigID                 string                 `json:"gig_id"`
	FreelancerID          string                 `json:"freelancer_id"`
	Slug                  string                 `json:"slug"`
	Title                 string                 `json:"title"`
	Description           string                 `json:"description"`
	CategoryID            int64                  `json:"category_id"`
	Currency              string                 `json:"currency"`
	Status                string                 `json:"status"`
	BasicInfoCompleted    bool                   `json:"basic_info_completed"`
	PackagesCompleted     bool                   `json:"packages_completed"`
	RequirementsCompleted bool                   `json:"requirements_completed"`
	MediaCompleted        bool                   `json:"media_completed"`
	PictureFileID         string                 `json:"picture_file_id"`
	PublishedAt           *time.Time             `json:"published_at,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	Packages              []GigPublishedPackage  `json:"packages"`
	Questions             []GigPublishedQuestion `json:"questions"`
	Media                 []GigPublishedMedia    `json:"media"`
}

// GigPublishedPackage describes a package entry embedded in GigPublishedEvent.
type GigPublishedPackage struct {
	ID           string `json:"id,omitempty"`
	GigID        string `json:"gig_id,omitempty"`
	Tier         string `json:"tier"`
	Description  string `json:"description"`
	DeliveryDays int32  `json:"delivery_days"`
	PriceCents   int64  `json:"price_cents"`
	SortOrder    int32  `json:"sort_order"`
}

// GigPublishedQuestion describes a requirement question embedded in
// GigPublishedEvent.
type GigPublishedQuestion struct {
	ID        string `json:"id,omitempty"`
	GigID     string `json:"gig_id,omitempty"`
	Content   string `json:"content"`
	SortOrder int32  `json:"sort_order"`
}

// GigPublishedMedia describes a media entry embedded in GigPublishedEvent.
type GigPublishedMedia struct {
	ID        string `json:"id,omitempty"`
	GigID     string `json:"gig_id,omitempty"`
	FileID    string `json:"file_id"`
	SortOrder int32  `json:"sort_order"`
}
