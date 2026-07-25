package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"time"

	"gig-service/internal/domain"
	"gig-service/internal/infra/write/yugabyte/mapper"
	"gig-service/internal/infra/write/yugabyte/model"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type uuidV7Arg struct{}

func (uuidV7Arg) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	parsed, err := uuid.Parse(s)
	return err == nil && parsed.Version() == 7
}

var _ = Describe("yugabyte repository", func() {
	var (
		db       *sql.DB
		mock     sqlmock.Sqlmock
		sqlxDB   *sqlx.DB
		repoSvc  domain.GigRepository
		repoImpl *repo
		lg       logging.Logger
		now      = time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC)
	)

	BeforeEach(func() {
		var err error
		lg, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		db, mock, err = sqlmock.New()
		Expect(err).NotTo(HaveOccurred())
		sqlxDB = sqlx.NewDb(db, "sqlmock")
		repoSvc, err = New(sqlxDB, NewPgErrorTranslator(), lg)
		Expect(err).NotTo(HaveOccurred())
		repoImpl = repoSvc.(*repo)
	})

	AfterEach(func() {
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("validates constructor dependencies", func() {
		repo, err := New(nil, NewPgErrorTranslator(), lg)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilYugaByteDB))

		repo, err = New(sqlxDB, nil, lg)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilDBErrorTranslator))

		repo, err = New(sqlxDB, NewPgErrorTranslator(), nil)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("creates a draft and loads the aggregate", func() {
		mock.ExpectQuery(regexp.QuoteMeta(createGigDraftQuery)).
			WithArgs(uuidV7Arg{}, "freelancer-1").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "", "", "", "", int64(0), "", domain.StatusDraft, false, false, false, false, "", nil, now, now))
		expectGigLoads(mock, "gig-1", nil, nil, nil)

		gig, err := repoSvc.CreateDraft(context.Background(), domain.CreateDraftParams{FreelancerID: "freelancer-1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.ID).To(Equal("gig-1"))
		Expect(gig.FreelancerID).To(Equal("freelancer-1"))
	})

	It("returns domain errors for create failures", func() {
		mock.ExpectQuery(regexp.QuoteMeta(createGigDraftQuery)).
			WithArgs(uuidV7Arg{}, "freelancer-1").
			WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: GigsPrimaryKeyConstraint})

		gig, err := repoSvc.CreateDraft(context.Background(), domain.CreateDraftParams{FreelancerID: "freelancer-1"})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(domain.ErrGigAlreadyExists))
	})

	It("updates basic info and reloads the aggregate", func() {
		mock.ExpectQuery(regexp.QuoteMeta(updateGigBasicInfoQuery)).
			WithArgs("gig-1", "Title", "short", "slug", "desc", int64(1001), "usd").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "slug", "Title", "short", "desc", int64(1001), "usd", domain.StatusDraft, true, false, false, false, "", nil, now, now))
		expectGigLoads(mock, "gig-1", nil, nil, nil)

		gig, err := repoSvc.UpdateBasicInfo(context.Background(), "gig-1", domain.UpdateBasicInfoParams{
			Title:       "Title",
			ShortInfo:   "short",
			Slug:        "slug",
			Description: "desc",
			CategoryID:  1001,
			Currency:    "usd",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.Slug).To(Equal("slug"))
	})

	It("replaces packages in a transaction", func() {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigPackagesQuery)).WithArgs("gig-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigPackageQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", domain.TierBasic, "basic", int32(1), int64(100), int32(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigPackageQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", domain.TierStandard, "standard", int32(2), int64(200), int32(2)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigPackageQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", domain.TierPremium, "premium", int32(3), int64(300), int32(3)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta("UPDATE gigs SET packages_completed = TRUE, updated_at = NOW() WHERE gig_id = $1")).WithArgs("gig-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuery)).WithArgs("gig-1").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "slug", "Title", "short", "desc", int64(1001), "usd", domain.StatusDraft, true, true, false, false, "file-1", nil, now, now))
		expectGigLoads(mock, "gig-1", []model.GigPackageRow{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}, {ID: "pkg-2", GigID: "gig-1", Tier: domain.TierStandard, Description: "standard", DeliveryDays: 2, PriceCents: 200, SortOrder: 2}, {ID: "pkg-3", GigID: "gig-1", Tier: domain.TierPremium, Description: "premium", DeliveryDays: 3, PriceCents: 300, SortOrder: 3}}, nil, nil)

		gig, err := repoSvc.ReplacePackages(context.Background(), "gig-1", domain.ReplacePackagesParams{
			Packages: []domain.GigPackage{
				{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100},
				{Tier: domain.TierStandard, Description: "standard", DeliveryDays: 2, PriceCents: 200},
				{Tier: domain.TierPremium, Description: "premium", DeliveryDays: 3, PriceCents: 300},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.Packages).To(HaveLen(3))
	})

	It("replaces questions in a transaction", func() {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigQuestionsQuery)).WithArgs("gig-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigQuestionQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", "question one", int32(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigQuestionQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", "question two", int32(2)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta("UPDATE gigs SET requirements_completed = TRUE, updated_at = NOW() WHERE gig_id = $1")).WithArgs("gig-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuery)).WithArgs("gig-1").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "slug", "Title", "short", "desc", int64(1001), "usd", domain.StatusDraft, true, true, true, false, "file-1", nil, now, now))
		expectGigLoads(mock, "gig-1", nil, []model.GigQuestionRow{{ID: "q-1", GigID: "gig-1", Content: "question one", SortOrder: 1}, {ID: "q-2", GigID: "gig-1", Content: "question two", SortOrder: 2}}, nil)

		gig, err := repoSvc.ReplaceQuestions(context.Background(), "gig-1", domain.ReplaceQuestionsParams{
			Questions: []domain.GigQuestion{{Content: "question one"}, {Content: "question two"}},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.Questions).To(HaveLen(2))
	})

	It("replaces media in a transaction", func() {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigMediaQuery)).WithArgs("gig-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigMediaQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", "file-1", int32(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(insertGigMediaQuery)).
			WithArgs(uuidV7Arg{}, "gig-1", "file-2", int32(2)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta("UPDATE gigs SET picture_file_id = $2, media_completed = TRUE, updated_at = NOW() WHERE gig_id = $1")).WithArgs("gig-1", "file-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuery)).WithArgs("gig-1").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "slug", "Title", "short", "desc", int64(1001), "usd", domain.StatusDraft, true, true, true, true, "file-1", nil, now, now))
		expectGigLoads(mock, "gig-1", nil, nil, []model.GigMediaRow{{ID: "media-1", GigID: "gig-1", FileID: "file-1", SortOrder: 1}, {ID: "media-2", GigID: "gig-1", FileID: "file-2", SortOrder: 2}})

		gig, err := repoSvc.ReplaceMedia(context.Background(), "gig-1", domain.ReplaceMediaParams{
			PictureFileID: "file-1",
			Media: []domain.GigMedia{
				{GigID: "gig-1", FileID: "file-1", SortOrder: 1},
				{GigID: "gig-1", FileID: "file-2", SortOrder: 2},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.Media).To(HaveLen(2))
	})

	It("returns not found on missing gig", func() {
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuery)).WithArgs("missing").WillReturnError(sql.ErrNoRows)

		gig, err := repoSvc.GetByID(context.Background(), "missing")
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(domain.ErrGigNotFound))
	})

	It("publishes a gig and reloads it", func() {
		mock.ExpectQuery(regexp.QuoteMeta(publishGigQuery)).
			WithArgs("gig-1").
			WillReturnRows(sqlmock.NewRows([]string{"gig_id", "freelancer_id", "seller_username", "slug", "title", "short_info", "description", "category_id", "currency", "status", "basic_info_completed", "packages_completed", "requirements_completed", "media_completed", "picture_file_id", "published_at", "created_at", "updated_at"}).
				AddRow("gig-1", "freelancer-1", "", "slug", "Title", "short", "desc", int64(1001), "usd", domain.StatusPublished, true, true, true, true, "file-1", now, now, now))
		expectGigLoads(mock, "gig-1", []model.GigPackageRow{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}}, []model.GigQuestionRow{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}}, []model.GigMediaRow{{ID: "media-1", GigID: "gig-1", FileID: "file-1", SortOrder: 1}})

		gig, err := repoSvc.Publish(context.Background(), "gig-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(gig.Status).To(Equal(domain.StatusPublished))
	})

	It("maps database errors with the translator", func() {
		Expect(NewPgErrorTranslator().TranslateCreateGigError(&pgconn.PgError{Code: pgerrcode.InvalidTextRepresentation})).To(MatchError(domain.ErrInvalidGigID))
		Expect(NewPgErrorTranslator().TranslateFindGigError(sql.ErrNoRows)).To(MatchError(domain.ErrGigNotFound))
		Expect(WrapCreateGigError(errors.New("boom"))).To(MatchError(ContainSubstring("create gig")))
		Expect(WrapFindGigError(errors.New("boom"))).To(MatchError(ContainSubstring("find gig")))
		Expect(WrapPublishGigError(errors.New("boom"))).To(MatchError(ContainSubstring("publish gig")))
		Expect(WrapDomainError(domain.ErrGigNotFound, errors.New("boom"))).To(MatchError(domain.ErrGigNotFound))
	})

	It("translates additional database error variants", func() {
		trans := NewPgErrorTranslator()
		Expect(trans.TranslateCreateGigError(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: GigsPrimaryKeyConstraint})).To(MatchError(domain.ErrGigAlreadyExists))
		Expect(trans.TranslateCreateGigError(errors.New("SQLSTATE " + pgerrcode.UniqueViolation))).To(MatchError(domain.ErrGigAlreadyExists))
		Expect(trans.TranslateCreateGigError(errors.New("SQLSTATE " + pgerrcode.InvalidTextRepresentation))).To(MatchError(domain.ErrInvalidGigID))
		Expect(trans.TranslateFindGigError(errors.New("boom"))).To(MatchError(ContainSubstring("find gig")))
		Expect(trans.TranslatePublishGigError(errors.New("boom"))).To(MatchError(ContainSubstring("publish gig")))
	})

	It("propagates load helper failures", func() {
		base := model.GigRow{ID: "gig-1", FreelancerID: "freelancer-1", Slug: "slug", Title: "Title", Description: "desc", CategoryID: 1001, Currency: "usd", Status: domain.StatusDraft, BasicInfoCompleted: true, PackagesCompleted: true, RequirementsCompleted: true, MediaCompleted: true, PictureFileID: "file-1", CreatedAt: now, UpdatedAt: now}

		mock.ExpectQuery(regexp.QuoteMeta(getGigPackagesQuery)).WithArgs("gig-1").WillReturnError(errors.New("packages"))
		gig, err := repoImpl.loadGig(context.Background(), "gig-1", base)
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("find gig")))

		mock.ExpectQuery(regexp.QuoteMeta(getGigPackagesQuery)).WithArgs("gig-1").WillReturnRows(gigPackagesRows(nil))
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuestionsQuery)).WithArgs("gig-1").WillReturnError(errors.New("questions"))
		gig, err = repoImpl.loadGig(context.Background(), "gig-1", base)
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("find gig")))

		mock.ExpectQuery(regexp.QuoteMeta(getGigPackagesQuery)).WithArgs("gig-1").WillReturnRows(gigPackagesRows(nil))
		mock.ExpectQuery(regexp.QuoteMeta(getGigQuestionsQuery)).WithArgs("gig-1").WillReturnRows(gigQuestionsRows(nil))
		mock.ExpectQuery(regexp.QuoteMeta(getGigMediaQuery)).WithArgs("gig-1").WillReturnError(errors.New("media"))
		gig, err = repoImpl.loadGig(context.Background(), "gig-1", base)
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("find gig")))
	})

	It("reports query and transaction failures", func() {
		mock.ExpectQuery(regexp.QuoteMeta(updateGigBasicInfoQuery)).
			WithArgs("gig-1", "Title", "short", "slug", "desc", int64(1001), "usd").
			WillReturnError(errors.New("update"))
		gig, err := repoSvc.UpdateBasicInfo(context.Background(), "gig-1", domain.UpdateBasicInfoParams{Title: "Title", ShortInfo: "short", Slug: "slug", Description: "desc", CategoryID: 1001, Currency: "usd"})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("find gig")))

		mock.ExpectBegin().WillReturnError(errors.New("begin"))
		gig, err = repoSvc.ReplacePackages(context.Background(), "gig-1", domain.ReplacePackagesParams{})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("publish gig")))

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigPackagesQuery)).WithArgs("gig-1").WillReturnError(errors.New("delete"))
		gig, err = repoSvc.ReplacePackages(context.Background(), "gig-1", domain.ReplacePackagesParams{})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("publish gig")))

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigQuestionsQuery)).WithArgs("gig-1").WillReturnError(errors.New("delete"))
		gig, err = repoSvc.ReplaceQuestions(context.Background(), "gig-1", domain.ReplaceQuestionsParams{})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("publish gig")))

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(deleteGigMediaQuery)).WithArgs("gig-1").WillReturnError(errors.New("delete"))
		gig, err = repoSvc.ReplaceMedia(context.Background(), "gig-1", domain.ReplaceMediaParams{})
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("publish gig")))

		mock.ExpectQuery(regexp.QuoteMeta(publishGigQuery)).WithArgs("gig-1").WillReturnError(errors.New("publish"))
		gig, err = repoSvc.Publish(context.Background(), "gig-1")
		Expect(gig).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("publish gig")))
	})

	It("maps row data to the domain aggregate", func() {
		when := time.Now().UTC()
		gig := mapper.MapGigRowToDomain(model.GigRow{
			ID:                    "gig-1",
			FreelancerID:          "freelancer-1",
			Slug:                  "slug",
			Title:                 "Title",
			Description:           "desc",
			CategoryID:            1001,
			Currency:              "usd",
			Status:                domain.StatusDraft,
			BasicInfoCompleted:    true,
			PackagesCompleted:     true,
			RequirementsCompleted: true,
			MediaCompleted:        true,
			PictureFileID:         "file-1",
			PublishedAt:           &when,
			CreatedAt:             when,
			UpdatedAt:             when,
		}, []model.GigPackageRow{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}}, []model.GigQuestionRow{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}}, []model.GigMediaRow{{ID: "media-1", GigID: "gig-1", FileID: "file-1", SortOrder: 1}})

		Expect(gig.ID).To(Equal("gig-1"))
		Expect(gig.Packages).To(HaveLen(1))
		Expect(gig.Questions).To(HaveLen(1))
		Expect(gig.Media).To(HaveLen(1))
	})
})

func expectGigLoads(mock sqlmock.Sqlmock, gigID string, packages []model.GigPackageRow, questions []model.GigQuestionRow, media []model.GigMediaRow) {
	mock.ExpectQuery(regexp.QuoteMeta(getGigPackagesQuery)).WithArgs(gigID).WillReturnRows(gigPackagesRows(packages))
	mock.ExpectQuery(regexp.QuoteMeta(getGigQuestionsQuery)).WithArgs(gigID).WillReturnRows(gigQuestionsRows(questions))
	mock.ExpectQuery(regexp.QuoteMeta(getGigMediaQuery)).WithArgs(gigID).WillReturnRows(gigMediaRows(media))
}

func gigPackagesRows(rows []model.GigPackageRow) *sqlmock.Rows {
	cols := []string{"package_id", "gig_id", "tier", "description", "delivery_days", "price_cents", "sort_order", "created_at", "updated_at"}
	out := sqlmock.NewRows(cols)
	for _, row := range rows {
		out.AddRow(row.ID, row.GigID, row.Tier, row.Description, row.DeliveryDays, row.PriceCents, row.SortOrder, row.CreatedAt, row.UpdatedAt)
	}
	return out
}

func gigQuestionsRows(rows []model.GigQuestionRow) *sqlmock.Rows {
	cols := []string{"question_id", "gig_id", "content", "sort_order", "created_at", "updated_at"}
	out := sqlmock.NewRows(cols)
	for _, row := range rows {
		out.AddRow(row.ID, row.GigID, row.Content, row.SortOrder, row.CreatedAt, row.UpdatedAt)
	}
	return out
}

func gigMediaRows(rows []model.GigMediaRow) *sqlmock.Rows {
	cols := []string{"gig_id", "file_id", "sort_order", "created_at", "updated_at"}
	out := sqlmock.NewRows(cols)
	for _, row := range rows {
		out.AddRow(row.GigID, row.FileID, row.SortOrder, row.CreatedAt, row.UpdatedAt)
	}
	return out
}
