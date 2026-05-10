package mapper

import (
	"gig-service/internal/domain"
	"gig-service/internal/infra/write/yugabyte/model"
)

// MapGigRowToDomain converts a write-model row into the domain aggregate.
func MapGigRowToDomain(row model.GigRow, packages []model.GigPackageRow, questions []model.GigQuestionRow, media []model.GigMediaRow) *domain.Gig {
	gig := &domain.Gig{
		ID:                    row.ID,
		FreelancerID:          row.FreelancerID,
		Slug:                  row.Slug,
		Title:                 row.Title,
		Description:           row.Description,
		CategoryID:            row.CategoryID,
		Currency:              row.Currency,
		Status:                row.Status,
		BasicInfoCompleted:    row.BasicInfoCompleted,
		PackagesCompleted:     row.PackagesCompleted,
		RequirementsCompleted: row.RequirementsCompleted,
		MediaCompleted:        row.MediaCompleted,
		PictureFileID:         row.PictureFileID,
		PublishedAt:           row.PublishedAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}

	if len(packages) > 0 {
		gig.Packages = make([]domain.GigPackage, 0, len(packages))
		for _, pkg := range packages {
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

	if len(questions) > 0 {
		gig.Questions = make([]domain.GigQuestion, 0, len(questions))
		for _, q := range questions {
			gig.Questions = append(gig.Questions, domain.GigQuestion{
				ID:        q.ID,
				GigID:     q.GigID,
				Content:   q.Content,
				SortOrder: q.SortOrder,
			})
		}
	}

	if len(media) > 0 {
		gig.Media = make([]domain.GigMedia, 0, len(media))
		for _, item := range media {
			gig.Media = append(gig.Media, domain.GigMedia{
				GigID:     item.GigID,
				FileID:    item.FileID,
				SortOrder: item.SortOrder,
			})
		}
	}

	return gig
}
