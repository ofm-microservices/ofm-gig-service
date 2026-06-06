package service

import (
	"context"
	"errors"
	"gig-service/config"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"strings"
	"time"
)

type gigService struct {
	repo               domain.GigRepository
	readRepo           domain.GigReadRepository
	files              FileService
	connect            ConnectStatusChecker
	review             ReviewClient
	orderCount         OrderCountClient
	broker             EventBroker
	pop                PopularitySource
	slug               Slugger
	mapr               GigEventMapper
	log                Logger
	previewPageSize    int
	previewWindowSize  int
	previewPagesWindow int
	previewWindowTTL   time.Duration
}

// New constructs the gig application service.
func New(repo domain.GigRepository, readRepo domain.GigReadRepository, files FileService, connect ConnectStatusChecker, review ReviewClient, orderCount OrderCountClient, broker EventBroker, pop PopularitySource, slugger Slugger, log Logger, previewCfg config.PreviewPaginationConfig) (GigService, error) {
	if repo == nil {
		return nil, ErrNilGigRepository
	}
	if readRepo == nil {
		return nil, ErrNilGigReadRepository
	}
	if files == nil {
		return nil, ErrNilFileService
	}
	if connect == nil {
		return nil, ErrNilConnectStatusChecker
	}
	if review == nil {
		return nil, ErrNilReviewClient
	}
	if orderCount == nil {
		return nil, ErrNilOrderCountClient
	}
	if broker == nil {
		return nil, ErrNilEventBroker
	}
	if pop == nil {
		return nil, ErrNilPopularitySource
	}
	if slugger == nil {
		return nil, ErrNilSlugger
	}
	if log == nil {
		return nil, ErrNilLogger
	}
	if previewCfg.PageSize <= 0 {
		return nil, ErrInvalidPreviewConfig
	}
	if previewCfg.WindowSize <= 0 {
		return nil, ErrInvalidPreviewConfig
	}
	if previewCfg.WindowSize%previewCfg.PageSize != 0 {
		return nil, ErrInvalidPreviewConfig
	}

	return &gigService{
		repo:               repo,
		readRepo:           readRepo,
		files:              files,
		connect:            connect,
		review:             review,
		orderCount:         orderCount,
		broker:             broker,
		pop:                pop,
		slug:               slugger,
		mapr:               NewGigEventMapper(),
		log:                log.With(logging.String("module", "application")),
		previewPageSize:    previewCfg.PageSize,
		previewWindowSize:  previewCfg.WindowSize,
		previewPagesWindow: previewCfg.WindowSize / previewCfg.PageSize,
		previewWindowTTL:   previewCfg.WindowTTL,
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
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
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
	shortInfo := strings.TrimSpace(params.ShortInfo)
	if shortInfo == "" {
		return nil, domain.ErrInvalidShortInfo
	}
	if len(shortInfo) > 128 {
		return nil, domain.ErrInvalidShortInfo
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
	slug := s.slug.Generate(title, gig.ID)
	if slug == "" {
		return nil, domain.ErrInvalidTitle
	}

	gig, err = s.repo.UpdateBasicInfo(ctx, gig.ID, domain.UpdateBasicInfoParams{
		Title:       title,
		ShortInfo:   shortInfo,
		Slug:        slug,
		Description: strings.TrimSpace(params.Description),
		CategoryID:  params.CategoryID,
		Currency:    strings.TrimSpace(params.Currency),
	})
	if err != nil {
		return nil, err
	}
	if err := s.publishProjection(ctx, gig); err != nil {
		return nil, err
	}
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
		return nil, err
	}

	return gig, nil
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

	gig, err = s.repo.ReplacePackages(ctx, gig.ID, params)
	if err != nil {
		return nil, err
	}
	if err := s.publishProjection(ctx, gig); err != nil {
		return nil, err
	}
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
		return nil, err
	}

	return gig, nil
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

	gig, err = s.repo.ReplaceQuestions(ctx, gig.ID, params)
	if err != nil {
		return nil, err
	}
	if err := s.publishProjection(ctx, gig); err != nil {
		return nil, err
	}
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
		return nil, err
	}

	return gig, nil
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
	if err := s.publishProjection(ctx, gig); err != nil {
		return nil, err
	}
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
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

func (s *gigService) GetPublicByID(ctx context.Context, gigID string) (*domain.Gig, error) {
	gigID = strings.TrimSpace(gigID)
	if gigID == "" {
		return nil, domain.ErrInvalidGigID
	}

	gig, err := s.readRepo.GetByID(ctx, gigID)
	if err == nil && gig != nil {
		if payload, err := s.mapr.ToViewedPayload(gig); err == nil {
			if err := s.broker.Publish(ctx, gigViewedSubject, payload); err != nil {
				s.log.Error("failed to publish gig viewed",
					logging.Operation("gig.viewed"),
					logging.String("gig_id", gig.ID),
					logging.String("slug", gig.Slug),
					logging.Err(err),
				)
			}
		} else {
			s.log.Error("failed to build gig viewed payload",
				logging.Operation("gig.viewed"),
				logging.String("gig_id", gig.ID),
				logging.String("slug", gig.Slug),
				logging.Err(err),
			)
		}
		return gig, nil
	}
	if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
		return nil, err
	}

	gig, err = s.repo.GetByID(ctx, gigID)
	if err != nil {
		return nil, err
	}
	if gig.Status != domain.StatusPublished {
		return nil, domain.ErrGigNotFound
	}

	gig, err = s.Project(ctx, gig)
	if err != nil {
		return nil, err
	}
	if err := s.readRepo.Upsert(ctx, gig); err != nil {
		return nil, err
	}

	if payload, err := s.mapr.ToViewedPayload(gig); err == nil {
		if err := s.broker.Publish(ctx, gigViewedSubject, payload); err != nil {
			s.log.Error("failed to publish gig viewed",
				logging.Operation("gig.viewed"),
				logging.String("gig_id", gig.ID),
				logging.String("slug", gig.Slug),
				logging.Err(err),
			)
		}
	} else {
		s.log.Error("failed to build gig viewed payload",
			logging.Operation("gig.viewed"),
			logging.String("gig_id", gig.ID),
			logging.String("slug", gig.Slug),
			logging.Err(err),
		)
	}

	return gig, nil
}

// RebuildPopularitySnapshots refreshes the popularity snapshots for the
// provided gigs using the current analytics aggregates.
func (s *gigService) RebuildPopularitySnapshots(ctx context.Context, gigs []*domain.Gig) error {
	if len(gigs) == 0 {
		return nil
	}

	rows, err := s.pop.ListPopularityRows(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]PopularityRow, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.GigID) == "" {
			continue
		}
		byID[row.GigID] = row
	}

	now := time.Now().UTC()
	for _, gig := range gigs {
		if gig == nil || strings.TrimSpace(gig.ID) == "" {
			continue
		}
		row := byID[gig.ID]
		score := row.CompletedOrdersLast30d*1000 + row.ReviewsCountLast30d*200 + row.ViewsLast7d*5
		if err := s.readRepo.SetPopularitySnapshot(ctx, &domain.GigPopularitySnapshot{
			GigID:                  gig.ID,
			Score:                  score,
			CompletedOrdersLast30d: row.CompletedOrdersLast30d,
			ReviewsCountLast30d:    row.ReviewsCountLast30d,
			ViewsLast7d:            row.ViewsLast7d,
			CalculatedAt:           now,
		}); err != nil {
			s.log.Error("refresh popularity snapshot failed",
				logging.Operation("gig.popularity.refresh"),
				logging.String("gig_id", gig.ID),
				logging.Err(err),
			)
		}
	}

	return nil
}

func (s *gigService) GetOrderStartSnapshot(ctx context.Context, gigID, packageID string) (*OrderStartSnapshot, error) {
	gigID = strings.TrimSpace(gigID)
	packageID = strings.TrimSpace(packageID)
	if gigID == "" {
		return nil, domain.ErrInvalidGigID
	}
	if packageID == "" {
		return nil, domain.ErrInvalidPackageTier
	}

	gig, err := s.repo.GetByID(ctx, gigID)
	if err != nil {
		return nil, err
	}
	if gig.Status != domain.StatusPublished {
		return nil, domain.ErrGigDraftIncomplete
	}
	var pkg *domain.GigPackage
	for i := range gig.Packages {
		if strings.TrimSpace(gig.Packages[i].ID) == packageID {
			pkg = &gig.Packages[i]
			break
		}
	}
	if pkg == nil {
		return nil, domain.ErrInvalidPackageTier
	}
	username := strings.TrimSpace(gig.SellerUsername)
	if username == "" && s.readRepo != nil {
		if cached, err := s.readRepo.GetByID(ctx, gig.ID); err == nil && cached != nil {
			username = strings.TrimSpace(cached.SellerUsername)
		}
	}
	if username != "" && strings.TrimSpace(gig.SellerUsername) == "" {
		if updated, err := s.repo.UpdateSellerUsername(ctx, gig.ID, username); err == nil && updated != nil {
			gig = updated
		}
	}

	snapshot := &OrderStartSnapshot{
		GigID:              gig.ID,
		PackageID:          pkg.ID,
		SellerID:           gig.FreelancerID,
		SellerUsername:     username,
		GigTitle:           gig.Title,
		PictureFileID:      gig.PictureFileID,
		PackageTitle:       pkg.Tier,
		PackageDescription: pkg.Description,
		PriceCents:         pkg.PriceCents,
		Currency:           gig.Currency,
		DeliveryDays:       pkg.DeliveryDays,
		RevisionCount:      int32(len(gig.Questions)),
		GigPublished:       gig.Status == domain.StatusPublished,
		PackageAvailable:   true,
	}
	if len(gig.Questions) > 0 {
		snapshot.Questions = make([]OrderStartQuestion, 0, len(gig.Questions))
		for _, q := range gig.Questions {
			snapshot.Questions = append(snapshot.Questions, OrderStartQuestion{
				ID:        q.ID,
				Text:      q.Content,
				SortOrder: q.SortOrder,
			})
		}
	}
	return snapshot, nil
}

func (s *gigService) Publish(ctx context.Context, gigID, freelancerID, username string) (*domain.Gig, error) {
	gig, err := s.getOwnedGig(ctx, gigID, freelancerID)
	if err != nil {
		return nil, err
	}
	if err := validatePublishReady(gig); err != nil {
		return nil, err
	}
	status, err := s.connect.GetConnectStatus(ctx, gig.FreelancerID)
	if err != nil {
		s.log.Error("failed to check connect onboarding status", logging.String("freelancer_id", gig.FreelancerID), logging.Err(err))
		return nil, err
	}
	if status == nil || status.Status != "completed" {
		return nil, domain.ErrConnectOnboardingIncomplete
	}

	gig, err = s.repo.Publish(ctx, gig.ID)
	if err != nil {
		s.log.Error("failed to publish gig", logging.String("gig_id", gig.ID), logging.Err(err))
		return nil, err
	}

	username = strings.TrimSpace(username)
	if username != "" && strings.TrimSpace(gig.SellerUsername) == "" {
		gig, err = s.repo.UpdateSellerUsername(ctx, gig.ID, username)
		if err != nil {
			return nil, err
		}
	}

	payload, err := s.mapr.ToPublishedPayload(gig)
	if err != nil {
		return nil, err
	}
	if err := s.broker.Publish(ctx, gigPublishedSubject, payload); err != nil {
		return nil, err
	}
	if err := s.publishProjection(ctx, gig); err != nil {
		return nil, err
	}
	if err := s.publishPreviewProjection(ctx, gig); err != nil {
		return nil, err
	}

	return gig, nil
}

func (s *gigService) Project(ctx context.Context, gig *domain.Gig) (*domain.Gig, error) {
	if gig == nil {
		return nil, domain.ErrGigNotFound
	}

	projected := *gig
	fileIDs := make([]string, 0, 1+len(gig.Media))
	seen := make(map[string]struct{}, 1+len(gig.Media))
	if picture := strings.TrimSpace(projected.PictureFileID); picture != "" {
		fileIDs = append(fileIDs, picture)
		seen[picture] = struct{}{}
	}
	for _, item := range gig.Media {
		if fileID := strings.TrimSpace(item.FileID); fileID != "" {
			if _, ok := seen[fileID]; ok {
				continue
			}
			seen[fileID] = struct{}{}
			fileIDs = append(fileIDs, fileID)
		}
	}

	urls := map[string]string{}
	if len(fileIDs) > 0 {
		resolved, err := s.files.GetFileURLs(ctx, fileIDs)
		if err == nil {
			urls = resolved
		}
	}
	if picture := strings.TrimSpace(projected.PictureFileID); picture != "" {
		projected.PictureURL = urls[picture]
	}

	if len(gig.Media) > 0 {
		projected.Media = make([]domain.GigMedia, 0, len(gig.Media))
		for _, item := range gig.Media {
			media := domain.GigMedia{
				GigID:     item.GigID,
				FileID:    item.FileID,
				SortOrder: item.SortOrder,
				URL:       urls[strings.TrimSpace(item.FileID)],
			}
			projected.Media = append(projected.Media, media)
		}
	}

	return &projected, nil
}

func (s *gigService) publishProjection(ctx context.Context, gig *domain.Gig) error {
	payload, err := s.mapr.ToPublishedPayload(gig)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, gigProjectionRequestedSubject, payload)
}

func (s *gigService) publishPreviewProjection(ctx context.Context, gig *domain.Gig) error {
	payload, err := s.mapr.ToPublishedPayload(gig)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, gigPreviewProjectionSubject, payload)
}

func (s *gigService) AppendPreviewGig(ctx context.Context, gig *domain.Gig) error {
	if gig == nil {
		return domain.ErrGigNotFound
	}

	if err := s.UpsertPreviewGig(ctx, gig); err != nil {
		return err
	}
	if gig.Status != domain.StatusPublished {
		return nil
	}

	current, err := s.readRepo.ListPreviewWindow(ctx, gig.FreelancerID, 0)
	if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
		return err
	}
	if current == nil || len(current.Gigs) == 0 {
		if _, err := s.readRepo.GetUserLookup(ctx, gig.SellerUsername); err != nil && !errors.Is(err, domain.ErrGigNotFound) {
			return err
		}
		current, err = s.readRepo.ListPreviewWindow(ctx, gig.FreelancerID, 0)
		if err != nil && !errors.Is(err, domain.ErrGigNotFound) {
			return err
		}
		if current != nil && len(current.Gigs) > 0 {
			preview := s.toPreview(gig, nil, nil, nil)
			preview.PopularityScore = 0
			return s.readRepo.AppendPreviewGig(ctx, gig.FreelancerID, preview, s.previewWindowSize, s.previewWindowTTL)
		}

		_, err := s.GetPreviewGigsByFreelancerUsername(ctx, domain.ListPreviewGigsQuery{
			SellerUsername: gig.SellerUsername,
			Page:           1,
			Limit:          s.previewPageSize,
		})
		if err != nil {
			return err
		}
	}

	preview := s.toPreview(gig, nil, nil, nil)
	preview.PopularityScore = 0
	return s.readRepo.AppendPreviewGig(ctx, gig.FreelancerID, preview, s.previewWindowSize, s.previewWindowTTL)
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
