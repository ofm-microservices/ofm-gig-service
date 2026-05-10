package domain

import (
	"context"
	"time"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusPaused    = "paused"
	StatusArchived  = "archived"

	TierBasic    = "basic"
	TierStandard = "standard"
	TierPremium  = "premium"
)

// Gig represents the write model owned by gig-service.
type Gig struct {
	ID                    string
	FreelancerID          string
	Slug                  string
	Title                 string
	Description           string
	CategoryID            int64
	Currency              string
	Status                string
	BasicInfoCompleted    bool
	PackagesCompleted     bool
	RequirementsCompleted bool
	MediaCompleted        bool
	PictureFileID         string
	PublishedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Packages              []GigPackage
	Questions             []GigQuestion
	Media                 []GigMedia
}

// GigPackage describes one pricing tier of a gig.
type GigPackage struct {
	ID           string
	GigID        string
	Tier         string
	Description  string
	DeliveryDays int32
	PriceCents   int64
	SortOrder    int32
}

// GigQuestion describes a pre-sale requirement question.
type GigQuestion struct {
	ID        string
	GigID     string
	Content   string
	SortOrder int32
}

// GigMedia describes one gallery media file attached to a gig.
type GigMedia struct {
	GigID     string
	FileID    string
	SortOrder int32
}

// MediaUpload carries one file body provided by the gateway.
type MediaUpload struct {
	Filename    string
	ContentType string
	Data        []byte
}

// CreateDraftParams identifies the freelancer starting a new draft.
type CreateDraftParams struct {
	FreelancerID string
}

// UpdateBasicInfoParams contains the core public gig metadata.
type UpdateBasicInfoParams struct {
	Title       string
	Slug        string
	Description string
	CategoryID  int64
	Currency    string
}

// ReplacePackagesParams replaces all gig packages at once.
type ReplacePackagesParams struct {
	Packages []GigPackage
}

// ReplaceQuestionsParams replaces all gig questions at once.
type ReplaceQuestionsParams struct {
	Questions []GigQuestion
}

// ReplaceMediaParams replaces the gig cover file and ordered gallery media
// references in one write-model update.
type ReplaceMediaParams struct {
	PictureFileID string
	Media         []GigMedia
}

// ReplaceMediaUploadParams carries the batch of files that should be uploaded
// before the gig write model is updated.
type ReplaceMediaUploadParams struct {
	Files []MediaUpload
}

// GigRepository persists the gig write model.
type GigRepository interface {
	CreateDraft(ctx context.Context, params CreateDraftParams) (*Gig, error)
	UpdateBasicInfo(ctx context.Context, gigID string, params UpdateBasicInfoParams) (*Gig, error)
	ReplacePackages(ctx context.Context, gigID string, params ReplacePackagesParams) (*Gig, error)
	ReplaceQuestions(ctx context.Context, gigID string, params ReplaceQuestionsParams) (*Gig, error)
	ReplaceMedia(ctx context.Context, gigID string, params ReplaceMediaParams) (*Gig, error)
	GetByID(ctx context.Context, gigID string) (*Gig, error)
	Publish(ctx context.Context, gigID string) (*Gig, error)
}

// GigReadRepository persists the gig read model in Redis.
type GigReadRepository interface {
	Upsert(ctx context.Context, gig *Gig) error
	DeleteByID(ctx context.Context, gigID string) error
}
