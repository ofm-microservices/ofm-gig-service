package service

import (
	"context"
	"gig-service/internal/domain"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
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
	DeleteFile(ctx context.Context, fileID string) error
}

// Slugger turns human-readable titles into URL-safe slugs for gig storage.
type Slugger interface {
	Generate(title string) string
}

// GigService owns gig draft creation, draft updates, and publication.
type GigService interface {
	CreateDraft(ctx context.Context, freelancerID string) (*domain.Gig, error)
	UpdateBasicInfo(ctx context.Context, gigID, freelancerID string, params domain.UpdateBasicInfoParams) (*domain.Gig, error)
	ReplacePackages(ctx context.Context, gigID, freelancerID string, params domain.ReplacePackagesParams) (*domain.Gig, error)
	ReplaceQuestions(ctx context.Context, gigID, freelancerID string, params domain.ReplaceQuestionsParams) (*domain.Gig, error)
	ReplaceMedia(ctx context.Context, gigID, freelancerID string, params domain.ReplaceMediaUploadParams) (*domain.Gig, error)
	GetByID(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error)
	Publish(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error)
}
