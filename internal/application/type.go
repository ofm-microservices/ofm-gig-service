package service

import (
	"context"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

// EventBroker publishes gig lifecycle events to the local NATS broker.
type EventBroker interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// Logger aliases the shared structured logger used by the application layer.
type Logger = logging.Logger

// FileService uploads gig media files through the file-service boundary.
type FileService interface {
	UploadFiles(ctx context.Context, ownerID, prefix string, files []domain.MediaUpload) ([]string, error)
	GetFileURL(ctx context.Context, fileID string) (string, error)
	GetFileURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
	DeleteFile(ctx context.Context, fileID string) error
}

// ConnectStatusChecker queries payment-service for a freelancer's Stripe
// Connect onboarding status.
type ConnectStatusChecker interface {
	GetConnectStatus(ctx context.Context, userID string) (*ConnectStatusResult, error)
}

// Slugger turns human-readable titles into URL-safe slugs for gig storage.
type Slugger interface {
	Generate(title, gigID string) string
}

// ConnectStatusResult carries the normalized Connect onboarding state.
type ConnectStatusResult struct {
	UserID          string
	Status          string
	StripeAccountID string
	DisabledReason  string
	OccurredAt      string
}

// UserPreview carries the compact user identity used to backfill seller
// usernames in gig-service.
type UserPreview struct {
	UserID      string
	Username    string
	DisplayName string
	AvatarID    string
	AvatarURL   string
}

// PopularityRow carries the aggregated analytics used to rebuild popularity
// snapshots from ClickHouse.
type PopularityRow struct {
	GigID                  string `json:"gig_id"`
	CompletedOrdersLast30d int64  `json:"completed_orders_last_30d"`
	ReviewsCountLast30d    int64  `json:"reviews_count_last_30d"`
	ViewsLast7d            int64  `json:"views_last_7d"`
}

// PopularitySource queries the analytics store for popularity aggregates.
type PopularitySource interface {
	ListPopularityRows(ctx context.Context) ([]PopularityRow, error)
}

// GigService owns gig draft creation, draft updates, and publication.
type GigService interface {
	CreateDraft(ctx context.Context, freelancerID string) (*domain.Gig, error)
	UpdateBasicInfo(ctx context.Context, gigID, freelancerID string, params domain.UpdateBasicInfoParams) (*domain.Gig, error)
	ReplacePackages(ctx context.Context, gigID, freelancerID string, params domain.ReplacePackagesParams) (*domain.Gig, error)
	ReplaceQuestions(ctx context.Context, gigID, freelancerID string, params domain.ReplaceQuestionsParams) (*domain.Gig, error)
	ReplaceMedia(ctx context.Context, gigID, freelancerID string, params domain.ReplaceMediaUploadParams) (*domain.Gig, error)
	GetByID(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error)
	GetPublicByID(ctx context.Context, gigID string) (*domain.Gig, error)
	GetPreviewGigsByFreelancerUsername(ctx context.Context, query domain.ListPreviewGigsQuery) (*domain.ListPreviewGigsResult, error)
	// AppendPreviewGig seeds the freelancer preview cache with a published gig
	// using the current preview window policy.
	AppendPreviewGig(ctx context.Context, gig *domain.Gig) error
	RebuildPopularitySnapshots(ctx context.Context, gigs []*domain.Gig) error
	GetOrderStartSnapshot(ctx context.Context, gigID, packageID string) (*OrderStartSnapshot, error)
	Publish(ctx context.Context, gigID, freelancerID, username string) (*domain.Gig, error)
	Project(ctx context.Context, gig *domain.Gig) (*domain.Gig, error)
}

// OrderStartSnapshot contains the published gig and package data required by
// the order saga to create the draft order.
type OrderStartSnapshot struct {
	GigID              string
	PackageID          string
	SellerID           string
	SellerUsername     string
	GigTitle           string
	PackageTitle       string
	PackageDescription string
	PriceCents         int64
	Currency           string
	DeliveryDays       int32
	RevisionCount      int32
	GigPublished       bool
	PackageAvailable   bool
	Questions          []OrderStartQuestion
}

// OrderStartQuestion represents one published gig requirement question.
type OrderStartQuestion struct {
	ID        string
	Text      string
	SortOrder int32
}
