package grpc

import (
	"errors"
	service "gig-service/internal/application"
	"gig-service/internal/domain"
	"time"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	gigv1 "github.com/ofm-microservices/ofm-common/proto/gig/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GigMapper translates between gRPC messages and the gig application model.
type GigMapper interface {
	ToCreateDraftResponse(gig *domain.Gig) *gigv1.CreateDraftResponse
	ToGigResponse(gig *domain.Gig) *gigv1.Gig
	ToOrderStartSnapshot(snapshot *service.OrderStartSnapshot) *gigv1.OrderStartSnapshot
	ToGetGigBySlugResponse(gig *domain.Gig) *gigv1.GetGigBySlugResponse
	ToGetPreviewGigsByFreelancerUsernameResponse(result *domain.ListPreviewGigsResult) *gigv1.GetPreviewGigsByFreelancerUsernameResponse
	ToGetMyGigsResponse(result *domain.ListMyGigsResult) *gigv1.GetMyGigsResponse
	ToUpdateBasicInfoParams(req *gigv1.UpdateBasicInfoRequest) domain.UpdateBasicInfoParams
	ToReplacePackagesParams(req *gigv1.ReplacePackagesRequest) domain.ReplacePackagesParams
	ToReplaceQuestionsParams(req *gigv1.ReplaceQuestionsRequest) domain.ReplaceQuestionsParams
	ToReplaceMediaParams(req *gigv1.ReplaceMediaRequest) domain.ReplaceMediaUploadParams
	ToError(err error) error
}

type gigMapper struct {
	log logging.Logger
}

const timeFormat = time.RFC3339Nano

func newGigMapper(log logging.Logger) GigMapper {
	return &gigMapper{log: log}
}

func (m *gigMapper) ToCreateDraftResponse(gig *domain.Gig) *gigv1.CreateDraftResponse {
	return &gigv1.CreateDraftResponse{Gig: m.ToGigResponse(gig)}
}

func (m *gigMapper) ToGigResponse(gig *domain.Gig) *gigv1.Gig {
	if gig == nil {
		return nil
	}

	resp := &gigv1.Gig{
		GigId:                 gig.ID,
		FreelancerId:          gig.FreelancerID,
		SellerUsername:        gig.SellerUsername,
		Slug:                  gig.Slug,
		Title:                 gig.Title,
		ShortInfo:             gig.ShortInfo,
		Description:           gig.Description,
		CategoryId:            gig.CategoryID,
		Currency:              gig.Currency,
		Status:                gig.Status,
		BasicInfoCompleted:    gig.BasicInfoCompleted,
		PackagesCompleted:     gig.PackagesCompleted,
		RequirementsCompleted: gig.RequirementsCompleted,
		MediaCompleted:        gig.MediaCompleted,
		PictureFileId:         gig.PictureFileID,
		PictureUrl:            gig.PictureURL,
		CreatedAt:             gig.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:             gig.UpdatedAt.UTC().Format(timeFormat),
	}
	if gig.PublishedAt != nil {
		resp.PublishedAt = gig.PublishedAt.UTC().Format(timeFormat)
	}

	if len(gig.Packages) > 0 {
		resp.Packages = make([]*gigv1.GigPackage, 0, len(gig.Packages))
		for _, pkg := range gig.Packages {
			resp.Packages = append(resp.Packages, &gigv1.GigPackage{
				Id:           pkg.ID,
				GigId:        pkg.GigID,
				Tier:         pkg.Tier,
				Description:  pkg.Description,
				DeliveryDays: pkg.DeliveryDays,
				PriceCents:   pkg.PriceCents,
				SortOrder:    pkg.SortOrder,
			})
		}
	}

	if len(gig.Questions) > 0 {
		resp.Questions = make([]*gigv1.GigQuestion, 0, len(gig.Questions))
		for _, q := range gig.Questions {
			resp.Questions = append(resp.Questions, &gigv1.GigQuestion{
				Id:        q.ID,
				GigId:     q.GigID,
				Content:   q.Content,
				SortOrder: q.SortOrder,
			})
		}
	}

	if len(gig.Media) > 0 {
		resp.Media = make([]*gigv1.GigMedia, 0, len(gig.Media))
		for _, item := range gig.Media {
			resp.Media = append(resp.Media, &gigv1.GigMedia{
				Id:        item.FileID,
				GigId:     item.GigID,
				FileId:    item.FileID,
				Url:       item.URL,
				SortOrder: item.SortOrder,
			})
		}
	}

	return resp
}

func (m *gigMapper) ToOrderStartSnapshot(snapshot *service.OrderStartSnapshot) *gigv1.OrderStartSnapshot {
	if snapshot == nil {
		return nil
	}
	resp := &gigv1.OrderStartSnapshot{
		GigId:              snapshot.GigID,
		PackageId:          snapshot.PackageID,
		SellerUserId:       snapshot.SellerID,
		SellerUsername:     snapshot.SellerUsername,
		GigTitle:           snapshot.GigTitle,
		PictureFileId:      snapshot.PictureFileID,
		PackageTitle:       snapshot.PackageTitle,
		PackageDescription: snapshot.PackageDescription,
		PriceCents:         snapshot.PriceCents,
		Currency:           snapshot.Currency,
		DeliveryDays:       snapshot.DeliveryDays,
		RevisionCount:      snapshot.RevisionCount,
		GigPublished:       snapshot.GigPublished,
		PackageAvailable:   snapshot.PackageAvailable,
	}
	if len(snapshot.Questions) > 0 {
		resp.Questions = make([]*gigv1.GigQuestion, 0, len(snapshot.Questions))
		for _, q := range snapshot.Questions {
			resp.Questions = append(resp.Questions, &gigv1.GigQuestion{
				Id:        q.ID,
				Content:   q.Text,
				SortOrder: q.SortOrder,
			})
		}
	}
	return resp
}

func (m *gigMapper) ToGetGigBySlugResponse(gig *domain.Gig) *gigv1.GetGigBySlugResponse {
	return &gigv1.GetGigBySlugResponse{Gig: m.toPublicGigResponse(gig)}
}

func (m *gigMapper) ToUpdateBasicInfoParams(req *gigv1.UpdateBasicInfoRequest) domain.UpdateBasicInfoParams {
	return domain.UpdateBasicInfoParams{
		Title:       req.GetTitle(),
		ShortInfo:   req.GetShortInfo(),
		Description: req.GetDescription(),
		CategoryID:  req.GetCategoryId(),
		Currency:    req.GetCurrency(),
	}
}

func (m *gigMapper) ToGetPreviewGigsByFreelancerUsernameResponse(result *domain.ListPreviewGigsResult) *gigv1.GetPreviewGigsByFreelancerUsernameResponse {
	if result == nil {
		return nil
	}
	resp := &gigv1.GetPreviewGigsByFreelancerUsernameResponse{
		Page:       int32(result.Page),
		Limit:      int32(result.Limit),
		TotalPages: int32(result.TotalPages),
	}
	if len(result.Gigs) > 0 {
		resp.Gigs = make([]*gigv1.GigPreview, 0, len(result.Gigs))
		for _, gig := range result.Gigs {
			if gig == nil {
				continue
			}
			resp.Gigs = append(resp.Gigs, &gigv1.GigPreview{
				GigId:             gig.ID,
				Slug:              gig.Slug,
				Title:             gig.Title,
				ShortInfo:         gig.ShortInfo,
				MinimumPriceCents: gig.MinimumPriceCents,
				PictureUrl:        gig.PictureURL,
				CreatedAt:         gig.CreatedAt.UTC().Format(timeFormat),
				Status:            gig.Status,
				PublishedAt:       formatTimePtr(gig.PublishedAt),
				UpdatedAt:         gig.UpdatedAt.UTC().Format(timeFormat),
				RatingAvg:         gig.RatingAvg,
				TotalReviews:      gig.TotalReviews,
				OrderCount:        gig.OrderCount,
			})
		}
	}
	return resp
}

func (m *gigMapper) ToGetMyGigsResponse(result *domain.ListMyGigsResult) *gigv1.GetMyGigsResponse {
	if result == nil {
		return nil
	}
	resp := &gigv1.GetMyGigsResponse{
		Page:       int32(result.Page),
		Limit:      int32(result.Limit),
		TotalPages: int32(result.TotalPages),
	}
	if len(result.Gigs) > 0 {
		resp.Gigs = make([]*gigv1.GigPreview, 0, len(result.Gigs))
		for _, gig := range result.Gigs {
			if gig == nil {
				continue
			}
			resp.Gigs = append(resp.Gigs, &gigv1.GigPreview{
				GigId:             gig.ID,
				Slug:              gig.Slug,
				Title:             gig.Title,
				ShortInfo:         gig.ShortInfo,
				MinimumPriceCents: gig.MinimumPriceCents,
				PictureUrl:        gig.PictureURL,
				CreatedAt:         gig.CreatedAt.UTC().Format(timeFormat),
				Status:            gig.Status,
				PublishedAt:       formatTimePtr(gig.PublishedAt),
				UpdatedAt:         gig.UpdatedAt.UTC().Format(timeFormat),
				RatingAvg:         gig.RatingAvg,
				TotalReviews:      gig.TotalReviews,
				OrderCount:        gig.OrderCount,
			})
		}
	}
	return resp
}

func (m *gigMapper) ToReplacePackagesParams(req *gigv1.ReplacePackagesRequest) domain.ReplacePackagesParams {
	packages := make([]domain.GigPackage, 0, len(req.GetPackages()))
	for _, pkg := range req.GetPackages() {
		packages = append(packages, domain.GigPackage{
			ID:           pkg.GetId(),
			GigID:        pkg.GetGigId(),
			Tier:         pkg.GetTier(),
			Description:  pkg.GetDescription(),
			DeliveryDays: pkg.GetDeliveryDays(),
			PriceCents:   pkg.GetPriceCents(),
			SortOrder:    pkg.GetSortOrder(),
		})
	}

	return domain.ReplacePackagesParams{Packages: packages}
}

func (m *gigMapper) ToReplaceQuestionsParams(req *gigv1.ReplaceQuestionsRequest) domain.ReplaceQuestionsParams {
	questions := make([]domain.GigQuestion, 0, len(req.GetQuestions()))
	for _, q := range req.GetQuestions() {
		questions = append(questions, domain.GigQuestion{
			ID:        q.GetId(),
			GigID:     q.GetGigId(),
			Content:   q.GetContent(),
			SortOrder: q.GetSortOrder(),
		})
	}

	return domain.ReplaceQuestionsParams{Questions: questions}
}

func (m *gigMapper) ToReplaceMediaParams(req *gigv1.ReplaceMediaRequest) domain.ReplaceMediaUploadParams {
	files := make([]domain.MediaUpload, 0, len(req.GetFiles()))
	for _, item := range req.GetFiles() {
		files = append(files, domain.MediaUpload{
			Filename:    item.GetFilename(),
			ContentType: item.GetContentType(),
			Data:        item.GetData(),
		})
	}

	return domain.ReplaceMediaUploadParams{Files: files}
}

func formatTimePtr(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.UTC().Format(timeFormat)
}

func (m *gigMapper) toPublicGigResponse(gig *domain.Gig) *gigv1.Gig {
	if gig == nil {
		return nil
	}

	resp := &gigv1.Gig{
		GigId:          gig.ID,
		FreelancerId:   gig.FreelancerID,
		SellerUsername: gig.SellerUsername,
		Slug:           gig.Slug,
		Title:          gig.Title,
		ShortInfo:      gig.ShortInfo,
		Description:    gig.Description,
		CategoryId:     gig.CategoryID,
		Currency:       gig.Currency,
		PictureUrl:     gig.PictureURL,
		CreatedAt:      gig.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:      gig.UpdatedAt.UTC().Format(timeFormat),
	}
	if gig.PublishedAt != nil {
		resp.PublishedAt = gig.PublishedAt.UTC().Format(timeFormat)
	}
	if len(gig.Packages) > 0 {
		resp.Packages = make([]*gigv1.GigPackage, 0, len(gig.Packages))
		for _, pkg := range gig.Packages {
			resp.Packages = append(resp.Packages, &gigv1.GigPackage{
				Id:           pkg.ID,
				GigId:        pkg.GigID,
				Tier:         pkg.Tier,
				Description:  pkg.Description,
				DeliveryDays: pkg.DeliveryDays,
				PriceCents:   pkg.PriceCents,
				SortOrder:    pkg.SortOrder,
			})
		}
	}
	if len(gig.Media) > 0 {
		resp.Media = make([]*gigv1.GigMedia, 0, len(gig.Media))
		for _, item := range gig.Media {
			resp.Media = append(resp.Media, &gigv1.GigMedia{
				GigId:     item.GigID,
				Url:       item.URL,
				SortOrder: item.SortOrder,
			})
		}
	}
	return resp
}

func (m *gigMapper) ToError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidGigID),
		errors.Is(err, domain.ErrInvalidGigSlug),
		errors.Is(err, domain.ErrInvalidFreelancerID),
		errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrInvalidShortInfo),
		errors.Is(err, domain.ErrInvalidDescription),
		errors.Is(err, domain.ErrInvalidCategoryID),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrInvalidPackageTier),
		errors.Is(err, domain.ErrInvalidPackageDescription),
		errors.Is(err, domain.ErrInvalidPackageDeliveryDays),
		errors.Is(err, domain.ErrInvalidPackagePriceCents),
		errors.Is(err, domain.ErrInvalidQuestionContent),
		errors.Is(err, domain.ErrInvalidMediaUpload),
		errors.Is(err, domain.ErrInvalidMediaRef),
		errors.Is(err, domain.ErrInvalidFileID),
		errors.Is(err, domain.ErrInvalidPackageCount):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrGigNotFound):
		return status.Error(codes.NotFound, "gig not found")
	case errors.Is(err, domain.ErrGigDraftIncomplete),
		errors.Is(err, domain.ErrGigAlreadyPublished),
		errors.Is(err, domain.ErrInvalidGigState),
		errors.Is(err, domain.ErrConnectOnboardingIncomplete):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		m.log.Error("gig request failed", logging.Err(err))
		return status.Error(codes.Internal, "internal server error")
	}
}
