package repository

import (
	"context"
	"database/sql"
	"gig-service/internal/domain"
	"gig-service/internal/infra/write/yugabyte/mapper"
	"gig-service/internal/infra/write/yugabyte/model"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
)

type repo struct {
	db         *sqlx.DB
	translator DBErrorTranslator
	log        logging.Logger
}

// New constructs the Yugabyte-backed gig repository.
func New(db *sqlx.DB, translator DBErrorTranslator, log logging.Logger) (domain.GigRepository, error) {
	if db == nil {
		return nil, ErrNilYugaByteDB
	}
	if translator == nil {
		return nil, ErrNilDBErrorTranslator
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &repo{db: db, translator: translator, log: log.With(logging.String("module", "yugabyte-repository"))}, nil
}

func (r *repo) CreateDraft(ctx context.Context, params domain.CreateDraftParams) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveDB("yugabyte", "create_draft", "gigs", status, time.Since(started)) }()

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
		&row.SellerUsername,
		&row.Slug,
		&row.Title,
		&row.ShortInfo,
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
		status = "error"
		r.log.Error("create draft failed",
			logging.Operation("db.gig.create_draft"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("freelancer_id", params.FreelancerID),
			logging.Err(err),
		)
		return nil, r.translator.TranslateCreateGigError(err)
	}

	gig, err := r.loadGig(ctx, row.ID, row)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) UpdateBasicInfo(ctx context.Context, gigID string, params domain.UpdateBasicInfoParams) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "update_basic_info", "gigs", status, time.Since(started))
	}()

	var row model.GigRow
	if err := r.db.QueryRowContext(ctx, updateGigBasicInfoQuery, gigID, params.Title, params.ShortInfo, params.Slug, params.Description, params.CategoryID, params.Currency).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.SellerUsername,
		&row.Slug,
		&row.Title,
		&row.ShortInfo,
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
		status = "error"
		r.log.Error("update basic info failed",
			logging.Operation("db.gig.update_basic_info"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("slug", params.Slug),
			logging.Err(err),
		)
		return nil, r.translator.TranslateFindGigError(err)
	}

	gig, err := r.loadGig(ctx, gigID, row)
	if err != nil {
		status = "error"
	}
	return gig, err
}

// UpdateSellerUsername backfills the denormalized seller username on the gig
// write model.
func (r *repo) UpdateSellerUsername(ctx context.Context, gigID, sellerUsername string) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "update_seller_username", "gigs", status, time.Since(started))
	}()

	var row model.GigRow
	if err := r.db.QueryRowContext(ctx, updateGigSellerUsernameQuery, gigID, sellerUsername).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.SellerUsername,
		&row.Slug,
		&row.Title,
		&row.ShortInfo,
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
		status = "error"
		r.log.Error("update seller username failed",
			logging.Operation("db.gig.update_seller_username"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("seller_username", sellerUsername),
			logging.Err(err),
		)
		return nil, r.translator.TranslateFindGigError(err)
	}

	gig, err := r.loadGig(ctx, gigID, row)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) ReplacePackages(ctx context.Context, gigID string, params domain.ReplacePackagesParams) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "replace_packages", "gig_packages", status, time.Since(started))
	}()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace packages failed",
			logging.Operation("db.gig.replace_packages"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "started")
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigPackagesQuery, gigID); err != nil {
		status = "error"
		r.log.Error("replace packages failed",
			logging.Operation("db.gig.replace_packages"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
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
			status = "error"
			r.log.Error("replace packages failed",
				logging.Operation("db.gig.replace_packages"),
				logging.Attempt(1),
				logging.Retryable(false),
				logging.DurationMS(time.Since(started)),
				logging.String("gig_id", gigID),
				logging.String("tier", string(row.Tier)),
				logging.Err(err),
			)
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET packages_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID); err != nil {
		status = "error"
		r.log.Error("replace packages failed",
			logging.Operation("db.gig.replace_packages"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace packages failed",
			logging.Operation("db.gig.replace_packages"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "success")

	gig, err := r.GetByID(ctx, gigID)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) ReplaceQuestions(ctx context.Context, gigID string, params domain.ReplaceQuestionsParams) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "replace_questions", "gig_questions", status, time.Since(started))
	}()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace questions failed",
			logging.Operation("db.gig.replace_questions"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "started")
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigQuestionsQuery, gigID); err != nil {
		status = "error"
		r.log.Error("replace questions failed",
			logging.Operation("db.gig.replace_questions"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
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
			status = "error"
			r.log.Error("replace questions failed",
				logging.Operation("db.gig.replace_questions"),
				logging.Attempt(1),
				logging.Retryable(false),
				logging.DurationMS(time.Since(started)),
				logging.String("gig_id", gigID),
				logging.Err(err),
			)
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET requirements_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID); err != nil {
		status = "error"
		r.log.Error("replace questions failed",
			logging.Operation("db.gig.replace_questions"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace questions failed",
			logging.Operation("db.gig.replace_questions"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "success")

	gig, err := r.GetByID(ctx, gigID)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) ReplaceMedia(ctx context.Context, gigID string, params domain.ReplaceMediaParams) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "replace_media", "gig_media", status, time.Since(started))
	}()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace media failed",
			logging.Operation("db.gig.replace_media"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("picture_file_id", params.PictureFileID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "started")
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteGigMediaQuery, gigID); err != nil {
		status = "error"
		r.log.Error("replace media failed",
			logging.Operation("db.gig.replace_media"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("picture_file_id", params.PictureFileID),
			logging.Err(err),
		)
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
			status = "error"
			r.log.Error("replace media failed",
				logging.Operation("db.gig.replace_media"),
				logging.Attempt(1),
				logging.Retryable(false),
				logging.DurationMS(time.Since(started)),
				logging.String("gig_id", gigID),
				logging.String("file_id", row.FileID),
				logging.Err(err),
			)
			return nil, r.translator.TranslatePublishGigError(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gigs SET picture_file_id = $2, media_completed = TRUE, updated_at = NOW() WHERE gig_id = $1`, gigID, params.PictureFileID); err != nil {
		status = "error"
		r.log.Error("replace media failed",
			logging.Operation("db.gig.replace_media"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("picture_file_id", params.PictureFileID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	if err := tx.Commit(); err != nil {
		status = "error"
		metrics.Global().IncDBTransaction("yugabyte", "error")
		r.log.Error("replace media failed",
			logging.Operation("db.gig.replace_media"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.String("picture_file_id", params.PictureFileID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}
	metrics.Global().IncDBTransaction("yugabyte", "success")

	gig, err := r.GetByID(ctx, gigID)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) GetByID(ctx context.Context, gigID string) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveDB("yugabyte", "get_by_id", "gigs", status, time.Since(started)) }()

	var row model.GigRow
	if err := r.db.GetContext(ctx, &row, getGigQuery, gigID); err != nil {
		status = "error"
		if err == sql.ErrNoRows {
			return nil, domain.ErrGigNotFound
		}
		r.log.Error("get by id failed",
			logging.Operation("db.gig.get_by_id"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslateFindGigError(err)
	}

	gig, err := r.loadGig(ctx, gigID, row)
	if err != nil {
		status = "error"
	}
	return gig, err
}

func (r *repo) Publish(ctx context.Context, gigID string) (*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveDB("yugabyte", "publish", "gigs", status, time.Since(started)) }()

	var row model.GigRow
	if err := r.db.QueryRowContext(ctx, publishGigQuery, gigID).Scan(
		&row.ID,
		&row.FreelancerID,
		&row.SellerUsername,
		&row.Slug,
		&row.Title,
		&row.ShortInfo,
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
		status = "error"
		r.log.Error("publish failed",
			logging.Operation("db.gig.publish"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, r.translator.TranslatePublishGigError(err)
	}

	gig, err := r.loadGig(ctx, gigID, row)
	if err != nil {
		status = "error"
	}
	return gig, err
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

// ListAll loads every gig owned by the service for bootstrap and maintenance
// jobs.
func (r *repo) ListAll(ctx context.Context) ([]*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.Global().ObserveDB("yugabyte", "list_all", "gigs", status, time.Since(started)) }()

	var rows []model.GigRow
	if err := r.db.SelectContext(ctx, &rows, listAllGigsQuery); err != nil {
		status = "error"
		r.log.Error("list all gigs failed",
			logging.Operation("db.gig.list_all"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.Err(err),
		)
		return nil, r.translator.TranslateFindGigError(err)
	}

	out := make([]*domain.Gig, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.MapGigRowToDomain(row, nil, nil, nil))
	}
	return out, nil
}

// ListPublishedBySellerUsername loads the published gigs for one freelancer
// username.
func (r *repo) ListPublishedBySellerUsername(ctx context.Context, query domain.ListPreviewGigsQuery) ([]*domain.Gig, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "list_by_seller_username", "gigs", status, time.Since(started))
	}()

	username := strings.TrimSpace(query.SellerUsername)
	if username == "" {
		return nil, domain.ErrInvalidUsername
	}

	var rows []model.GigRow
	if err := r.db.SelectContext(ctx, &rows, listPublishedBySellerUsernameQuery, username); err != nil {
		status = "error"
		r.log.Error("list published gigs by seller username failed",
			logging.Operation("db.gig.list_published_by_seller_username"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("seller_username", username),
			logging.Err(err),
		)
		return nil, r.translator.TranslateFindGigError(err)
	}

	limit := query.Limit
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]*domain.Gig, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		gig, err := r.loadGig(ctx, row.ID, row)
		if err != nil {
			status = "error"
			return nil, err
		}
		out = append(out, gig)
	}

	return out, nil
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
