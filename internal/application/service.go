package service

import (
	"context"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"strings"
)

type gigService struct {
	repo   domain.GigRepository
	files  FileService
	broker EventBroker
	slug   Slugger
	mapr   GigEventMapper
	log    Logger
}

// New constructs the gig application service.
func New(repo domain.GigRepository, files FileService, broker EventBroker, slugger Slugger, log Logger) (GigService, error) {
	if repo == nil {
		return nil, ErrNilGigRepository
	}
	if files == nil {
		return nil, ErrNilFileService
	}
	if broker == nil {
		return nil, ErrNilEventBroker
	}
	if slugger == nil {
		return nil, ErrNilSlugger
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &gigService{
		repo:   repo,
		files:  files,
		broker: broker,
		slug:   slugger,
		mapr:   NewGigEventMapper(),
		log:    log.With(logging.String("module", "application")),
	}, nil
}

func (s *gigService) CreateDraft(ctx context.Context, freelancerID string) (*domain.Gig, error) {
	freelancerID = strings.TrimSpace(freelancerID)
	if freelancerID == "" {
		return nil, domain.ErrInvalidFreelancerID
	}

	gig, err := s.repo.CreateDraft(ctx, domain.CreateDraftParams{FreelancerID: freelancerID})
	if err != nil {
		s.log.Error("failed to create gig draft", logging.String("freelancer_id", freelancerID), logging.Err(err))
		return nil, err
	}

	return gig, nil
}

func (s *gigService) UpdateBasicInfo(ctx context.Context, gigID, freelancerID string, params domain.UpdateBasicInfoParams) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return nil, domain.ErrInvalidTitle
	}
	if params.Description == "" {
		return nil, domain.ErrInvalidDescription
	}
	if params.CategoryID <= 0 {
		return nil, domain.ErrInvalidCategoryID
	}
	if strings.TrimSpace(params.Currency) == "" {
		return nil, domain.ErrInvalidCurrency
	}
	slug := s.slug.Generate(title)
	if slug == "" {
		return nil, domain.ErrInvalidTitle
	}

	return s.repo.UpdateBasicInfo(ctx, gig.ID, domain.UpdateBasicInfoParams{
		Title:       title,
		Slug:        slug,
		Description: strings.TrimSpace(params.Description),
		CategoryID:  params.CategoryID,
		Currency:    strings.TrimSpace(params.Currency),
	})
}

func (s *gigService) ReplacePackages(ctx context.Context, gigID, freelancerID string, params domain.ReplacePackagesParams) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	normalizedPackages := make([]domain.GigPackage, 0, len(params.Packages))
	for i, pkg := range params.Packages {
		tier := strings.TrimSpace(pkg.Tier)
		if tier == "" {
			return nil, domain.ErrInvalidPackageTier
		}
		normalizedPackages = append(normalizedPackages, domain.GigPackage{
			ID:           strings.TrimSpace(pkg.ID),
			GigID:        strings.TrimSpace(pkg.GigID),
			Tier:         tier,
			Description:  strings.TrimSpace(pkg.Description),
			DeliveryDays: pkg.DeliveryDays,
			PriceCents:   pkg.PriceCents,
			SortOrder:    int32(i + 1),
		})
	}
	params.Packages = normalizedPackages
	if err := validatePackages(params.Packages); err != nil {
		return nil, err
	}

	return s.repo.ReplacePackages(ctx, gig.ID, params)
}

func (s *gigService) ReplaceQuestions(ctx context.Context, gigID, freelancerID string, params domain.ReplaceQuestionsParams) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	for _, q := range params.Questions {
		if strings.TrimSpace(q.Content) == "" {
			return nil, domain.ErrInvalidQuestionContent
		}
	}

	return s.repo.ReplaceQuestions(ctx, gig.ID, params)
}

func (s *gigService) ReplaceMedia(ctx context.Context, gigID, freelancerID string, params domain.ReplaceMediaUploadParams) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	if err := validateMediaUploads(params.Files); err != nil {
		return nil, err
	}

	fileIDs, err := s.files.UploadFiles(ctx, gig.FreelancerID, gigMediaPrefix(gig.ID), params.Files)
	if err != nil {
		return nil, err
	}

	if len(fileIDs) == 0 || len(fileIDs) != len(params.Files) {
		return nil, domain.ErrInvalidMediaUpload
	}

	gig, err = s.repo.ReplaceMedia(ctx, gig.ID, domain.ReplaceMediaParams{
		PictureFileID: fileIDs[0],
		Media:         buildGigMedia(gig.ID, fileIDs[1:]),
	})
	if err != nil {
		s.compensateUploadedFiles(ctx, fileIDs)
		return nil, err
	}

	return gig, nil
}

func (s *gigService) GetByID(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}

	return gig, nil
}

func (s *gigService) Publish(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	if err := validatePublishReady(gig); err != nil {
		return nil, err
	}

	gig, err = s.repo.Publish(ctx, gig.ID)
	if err != nil {
		s.log.Error("failed to publish gig", logging.String("gig_id", gig.ID), logging.Err(err))
		return nil, err
	}

	payload, err := s.mapr.ToPublishedPayload(gig)
	if err != nil {
		return nil, err
	}
	if err := s.broker.Publish(ctx, gigPublishedSubject, payload); err != nil {
		return nil, err
	}

	return gig, nil
}

func (s *gigService) getOwnedGig(ctx context.Context, gigID, freelancerID string) (*domain.Gig, error) {
	gigID = strings.TrimSpace(gigID)
	freelancerID = strings.TrimSpace(freelancerID)
	if gigID == "" {
		return nil, domain.ErrInvalidGigID
	}
	if freelancerID == "" {
		return nil, domain.ErrInvalidFreelancerID
	}

	gig, err := s.repo.GetByID(ctx, gigID)
	if err != nil {
		return nil, err
	}
	if gig.FreelancerID != freelancerID {
		return nil, domain.ErrInvalidFreelancerID
	}

	return gig, nil
}

func validatePackages(packages []domain.GigPackage) error {
	if len(packages) < 1 || len(packages) > 3 {
		return domain.ErrInvalidPackageCount
	}

	expected := []string{domain.TierBasic, domain.TierStandard, domain.TierPremium}
	for i, pkg := range packages {
		if strings.TrimSpace(pkg.Tier) != expected[i] {
			return domain.ErrInvalidPackageTier
		}
		if strings.TrimSpace(pkg.Description) == "" {
			return domain.ErrInvalidPackageDescription
		}
		if pkg.DeliveryDays <= 0 {
			return domain.ErrInvalidPackageDeliveryDays
		}
		if pkg.PriceCents <= 0 {
			return domain.ErrInvalidPackagePriceCents
		}
	}

	return nil
}

func gigMediaPrefix(gigID string) string {
	return "gigs/" + strings.TrimSpace(gigID) + "/media"
}

func (s *gigService) compensateUploadedFiles(ctx context.Context, fileIDs []string) {
	for i := len(fileIDs) - 1; i >= 0; i-- {
		_ = s.files.DeleteFile(ctx, fileIDs[i])
	}
}

func validatePublishReady(gig *domain.Gig) error {
	switch {
	case gig.Status != domain.StatusDraft:
		return domain.ErrGigAlreadyPublished
	case !gig.BasicInfoCompleted:
		return domain.ErrGigDraftIncomplete
	case !gig.PackagesCompleted:
		return domain.ErrGigDraftIncomplete
	case len(gig.Packages) == 0:
		return domain.ErrGigDraftIncomplete
	}
	if err := validatePackages(gig.Packages); err != nil {
		return err
	}

	return nil
}

func validateMediaUploads(files []domain.MediaUpload) error {
	if len(files) == 0 {
		return domain.ErrInvalidMediaUpload
	}
	for _, file := range files {
		if strings.TrimSpace(file.Filename) == "" {
			return domain.ErrInvalidMediaUpload
		}
		if strings.TrimSpace(file.ContentType) == "" {
			return domain.ErrInvalidMediaUpload
		}
		if len(file.Data) == 0 {
			return domain.ErrInvalidMediaUpload
		}
	}

	return nil
}

func buildGigMedia(gigID string, fileIDs []string) []domain.GigMedia {
	media := make([]domain.GigMedia, 0, len(fileIDs))
	for i, fileID := range fileIDs {
		media = append(media, domain.GigMedia{
			GigID:     gigID,
			FileID:    fileID,
			SortOrder: int32(i + 1),
		})
	}

	return media
}
