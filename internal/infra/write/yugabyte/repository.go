package repository

import (
	"context"
	"database/sql"
	"gig-service/internal/domain"
	"gig-service/internal/infra/write/yugabyte/mapper"
	"gig-service/internal/infra/write/yugabyte/model"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type repo struct {
	db         *sqlx.DB
	translator DBErrorTranslator
}

// New constructs the Yugabyte-backed gig repository.
func New(db *sqlx.DB, translator DBErrorTranslator) (domain.GigRepository, error) {
	if db == nil {
		return nil, ErrNilYugaByteDB
	}
	if translator == nil {
		return nil, ErrNilDBErrorTranslator
	}

	return &repo{db: db, translator: translator}, nil
}

func (r *repo) CreateDraft(ctx context.Context, params domain.CreateDraftParams) (*domain.Gig, error) {
	row := model.GigRow{
		ID:                    uuid.Must(uuid.NewV7()).String(),
		FreelancerID:          params.FreelancerID,
		Slug:                  "",
		Status:                domain.StatusDraft,
		PictureFileID:         "",
		BasicInfoCompleted:    false,
		PackagesCompleted:     false,
		RequirementsCompleted: false,
		MediaCompleted:        false,
	}

	if err := r.db.QueryRowContext(ctx, createGigDraftQuery, row.ID, row.FreelancerID).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.Slug,
		&row.Title,
		&row.Description,
		&row.CategoryID,
		&row.Currency,
		&row.Status,
		&row.BasicInfoCompleted,
		&row.PackagesCompleted,
		&row.RequirementsCompleted,
		&row.MediaCompleted,
		&row.PictureFileID,
		&row.PublishedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, r.translator.TranslateCreateGigError(err)
	}

	return r.loadGig(ctx, row.ID, row)
}

func (r *repo) UpdateBasicInfo(ctx context.Context, gigID string, params domain.UpdateBasicInfoParams) (*domain.Gig, error) {
	var row model.GigRow
	if err := r.db.QueryRowContext(ctx, updateGigBasicInfoQuery, gigID, params.Title, params.Slug, params.Description, params.CategoryID, params.Currency).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.Slug,
		&row.Title,
		&row.Description,
		&row.CategoryID,
		&row.Currency,
		&row.Status,
		&row.BasicInfoCompleted,
		&row.PackagesCompleted,
		&row.RequirementsCompleted,
		&row.MediaCompleted,
		&row.PictureFileID,
		&row.PublishedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, r.translator.TranslateFindGigError(err)
	}

	return r.loadGig(ctx, gigID, row)
}

func (r *repo) ReplacePackages(ctx context.Context, gigID string, params domain.ReplacePackagesParams) (*domain.Gig, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigPackagesQuery, gigID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	for i, pkg := range params.Packages {
		row := model.GigPackageRow{
			ID:           uuid.Must(uuid.NewV7()).String(),
			GigID:        gigID,
			Tier:         pkg.Tier,
			Description:  pkg.Description,
			DeliveryDays: pkg.DeliveryDays,
			PriceCents:   pkg.PriceCents,
			SortOrder:    int32(i + 1),
		}
		if _, err := tx.ExecContext(ctx, insertGigPackageQuery, row.ID, row.GigID, row.Tier, row.Description, row.DeliveryDays, row.PriceCents, row.SortOrder); err != nil {
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET packages_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}

	return r.GetByID(ctx, gigID)
}

func (r *repo) ReplaceQuestions(ctx context.Context, gigID string, params domain.ReplaceQuestionsParams) (*domain.Gig, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigQuestionsQuery, gigID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	for i, q := range params.Questions {
		row := model.GigQuestionRow{
			ID:        uuid.Must(uuid.NewV7()).String(),
			GigID:     gigID,
			Content:   q.Content,
			SortOrder: int32(i + 1),
		}
		if _, err := tx.ExecContext(ctx, insertGigQuestionQuery, row.ID, row.GigID, row.Content, row.SortOrder); err != nil {
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET requirements_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}

	return r.GetByID(ctx, gigID)
}

func (r *repo) ReplaceMedia(ctx context.Context, gigID string, params domain.ReplaceMediaParams) (*domain.Gig, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigMediaQuery, gigID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	for i, m := range params.Media {
		row := model.GigMediaRow{
			ID:        uuid.Must(uuid.NewV7()).String(),
			GigID:     gigID,
			FileID:    m.FileID,
			SortOrder: int32(i + 1),
		}
		if _, err := tx.ExecContext(ctx, insertGigMediaQuery, row.ID, row.GigID, row.FileID, row.SortOrder); err != nil {
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET picture_file_id = $2, media_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID, params.PictureFileID); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}

	return r.GetByID(ctx, gigID)
}

func (r *repo) GetByID(ctx context.Context, gigID string) (*domain.Gig, error) {
	var row model.GigRow
	if err := r.db.GetContext(ctx, &row, getGigQuery, gigID); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrGigNotFound
		}
		return nil, r.translator.TranslateFindGigError(err)
	}

	return r.loadGig(ctx, gigID, row)
}

func (r *repo) Publish(ctx context.Context, gigID string) (*domain.Gig, error) {
	var row model.GigRow
	if err := r.db.QueryRowContext(ctx, publishGigQuery, gigID).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.Slug,
		&row.Title,
		&row.Description,
		&row.CategoryID,
		&row.Currency,
		&row.Status,
		&row.BasicInfoCompleted,
		&row.PackagesCompleted,
		&row.RequirementsCompleted,
		&row.MediaCompleted,
		&row.PictureFileID,
		&row.PublishedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, r.translator.TranslatePublishGigError(err)
	}

	return r.loadGig(ctx, gigID, row)
}

func (r *repo) loadGig(ctx context.Context, gigID string, row model.GigRow) (*domain.Gig, error) {
	packages, err := r.loadPackages(ctx, gigID)
	if err != nil {
		return nil, err
	}
	questions, err := r.loadQuestions(ctx, gigID)
	if err != nil {
		return nil, err
	}
	media, err := r.loadMedia(ctx, gigID)
	if err != nil {
		return nil, err
	}

	return mapper.MapGigRowToDomain(row, packages, questions, media), nil
}

func (r *repo) loadPackages(ctx context.Context, gigID string) ([]model.GigPackageRow, error) {
	var rows []model.GigPackageRow
	if err := r.db.SelectContext(ctx, &rows, getGigPackagesQuery, gigID); err != nil {
		return nil, r.translator.TranslateFindGigError(err)
	}
	return rows, nil
}

func (r *repo) loadQuestions(ctx context.Context, gigID string) ([]model.GigQuestionRow, error) {
	var rows []model.GigQuestionRow
	if err := r.db.SelectContext(ctx, &rows, getGigQuestionsQuery, gigID); err != nil {
		return nil, r.translator.TranslateFindGigError(err)
	}
	return rows, nil
}

func (r *repo) loadMedia(ctx context.Context, gigID string) ([]model.GigMediaRow, error) {
	var rows []model.GigMediaRow
	if err := r.db.SelectContext(ctx, &rows, getGigMediaQuery, gigID); err != nil {
		return nil, r.translator.TranslateFindGigError(err)
	}
	return rows, nil
}
