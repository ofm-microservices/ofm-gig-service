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
	SellerUsername        string
	Slug                  string
	Title                 string
	ShortInfo             string
	Description           string
	CategoryID            int64
	Currency              string
	Status                string
	BasicInfoCompleted    bool
	PackagesCompleted     bool
	RequirementsCompleted bool
	MediaCompleted        bool
	PictureFileID         string
	PictureURL            string
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
	URL       string
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
	ShortInfo   string
	Description string
	CategoryID  int64
	Currency    string
}

// GigPreview describes the public freelancer gig list projection.
type GigPreview struct {
	ID                string     `json:"id"`
	FreelancerID      string     `json:"freelancer_id"`
	SellerUsername    string     `json:"seller_username"`
	Slug              string     `json:"slug"`
	Title             string     `json:"title"`
	ShortInfo         string     `json:"short_info"`
	MinimumPriceCents int64      `json:"minimum_price_cents"`
	PictureURL        string     `json:"picture_url"`
	PopularityScore   int64      `json:"popularity_score"`
	Status            string     `json:"status"`
	PublishedAt       *time.Time `json:"published_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RatingAvg         float64    `json:"rating_avg"`
	TotalReviews      int64      `json:"total_reviews"`
	OrderCount        int64      `json:"order_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

// GigPopularitySnapshot captures the materialized analytics snapshot used by
// freelancer preview ranking.
type GigPopularitySnapshot struct {
	GigID                  string    `json:"gig_id"`
	Score                  int64     `json:"score"`
	CompletedOrdersLast30d int64     `json:"completed_orders_last_30d"`
	ReviewsCountLast30d    int64     `json:"reviews_count_last_30d"`
	ViewsLast7d            int64     `json:"views_last_7d"`
	CalculatedAt           time.Time `json:"calculated_at"`
}

// ListPreviewGigsQuery describes a freelancer preview lookup.
type ListPreviewGigsQuery struct {
	SellerUsername string
	Page           int
	Limit          int
}

// ListMyGigsQuery describes the authenticated owner gig list lookup.
type ListMyGigsQuery struct {
	UserID string
	Status string
	Sort   string
	Order  string
	Page   int
	Limit  int
}

// ListPreviewGigsResult contains one page of freelancer gig previews.
type ListPreviewGigsResult struct {
	Gigs       []*GigPreview
	Page       int
	Limit      int
	TotalPages int
	HasMore    bool
}

// ListMyGigsResult contains one page of owner gig previews.
type ListMyGigsResult struct {
	Gigs       []*GigPreview
	Page       int
	Limit      int
	TotalPages int
	HasMore    bool
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
	UpdateSellerUsername(ctx context.Context, gigID, sellerUsername string) (*Gig, error)
	ReplacePackages(ctx context.Context, gigID string, params ReplacePackagesParams) (*Gig, error)
	ReplaceQuestions(ctx context.Context, gigID string, params ReplaceQuestionsParams) (*Gig, error)
	ReplaceMedia(ctx context.Context, gigID string, params ReplaceMediaParams) (*Gig, error)
	GetByID(ctx context.Context, gigID string) (*Gig, error)
	ListPublishedBySellerUsername(ctx context.Context, query ListPreviewGigsQuery) ([]*Gig, error)
	ListAll(ctx context.Context) ([]*Gig, error)
	Publish(ctx context.Context, gigID string) (*Gig, error)
}

// GigReadRepository persists the gig read model in Redis.
type GigReadRepository interface {
	Upsert(ctx context.Context, gig *Gig) error
	GetByID(ctx context.Context, gigID string) (*Gig, error)
	DeleteByID(ctx context.Context, gigID string) error
	UpsertPreviewWindow(ctx context.Context, userID string, window int, gigs []*GigPreview, hasMore bool, ttl time.Duration) error
	UpsertOwnerPreview(ctx context.Context, userID string, gig *GigPreview) error
	// AppendPreviewGig appends one published gig into the seller preview cache
	// using the current tail-window policy.
	AppendPreviewGig(ctx context.Context, userID string, gig *GigPreview, windowSize int, ttl time.Duration) error
	ListPreviewWindow(ctx context.Context, userID string, window int) (*ListPreviewGigsResult, error)
	ListOwnerPreviewGigs(ctx context.Context, userID string) ([]*GigPreview, error)
	GetPreviewByID(ctx context.Context, gigID string) (*GigPreview, error)
	SetUserLookup(ctx context.Context, username, userID string, ttl time.Duration) error
	GetUserLookup(ctx context.Context, username string) (string, error)
	SetPopularitySnapshot(ctx context.Context, snapshot *GigPopularitySnapshot) error
	GetPopularitySnapshot(ctx context.Context, gigID string) (*GigPopularitySnapshot, error)
	ListPopularitySnapshotsByGigIDs(ctx context.Context, gigIDs []string) (map[string]*GigPopularitySnapshot, error)
	ListPopularitySnapshots(ctx context.Context) ([]*GigPopularitySnapshot, error)
}
