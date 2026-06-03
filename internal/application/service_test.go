package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gig-service/config"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type fakePopularitySource struct{}

func (fakePopularitySource) ListPopularityRows(context.Context) ([]PopularityRow, error) {
	return nil, nil
}

func TestService(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gig Service Suite")
}

var _ = Describe("gigService", func() {
	var (
		ctrl     *gomock.Controller
		repo     *MockGigRepository
		readRepo *MockGigReadRepository
		files    *MockFileService
		connect  *MockConnectStatusChecker
		broker   *MockEventBroker
		pop      fakePopularitySource
		slugger  *MockSlugger
		logger   logging.Logger
		svc      GigService
		ctx      context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		repo = NewMockGigRepository(ctrl)
		readRepo = NewMockGigReadRepository(ctrl)
		files = NewMockFileService(ctrl)
		connect = NewMockConnectStatusChecker(ctrl)
		broker = NewMockEventBroker(ctrl)
		pop = fakePopularitySource{}
		slugger = NewMockSlugger(ctrl)

		var err error
		logger, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		svc, err = New(repo, readRepo, files, connect, broker, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
		Expect(err).NotTo(HaveOccurred())
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("New", func() {
		It("validates nil dependencies", func() {
			created, err := New(nil, readRepo, files, connect, broker, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilGigRepository))

			created, err = New(repo, nil, files, connect, broker, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilGigReadRepository))

			created, err = New(repo, readRepo, nil, connect, broker, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilFileService))

			created, err = New(repo, readRepo, files, nil, broker, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilConnectStatusChecker))

			created, err = New(repo, readRepo, files, connect, nil, pop, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilEventBroker))

			created, err = New(repo, readRepo, files, connect, broker, nil, slugger, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilPopularitySource))

			created, err = New(repo, readRepo, files, connect, broker, pop, nil, logger, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilSlugger))

			created, err = New(repo, readRepo, files, connect, broker, pop, slugger, nil, config.PreviewPaginationConfig{PageSize: 1, WindowSize: 2})
			Expect(created).To(BeNil())
			Expect(err).To(MatchError(ErrNilLogger))
		})
	})

	Describe("CreateDraft", func() {
		It("validates freelancer id", func() {
			result, err := svc.CreateDraft(ctx, "   ")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidFreelancerID))
		})

		It("creates a draft through the repository", func() {
			repo.EXPECT().
				CreateDraft(gomock.Any(), domain.CreateDraftParams{FreelancerID: "freelancer-1"}).
				Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

			result, err := svc.CreateDraft(ctx, " freelancer-1 ")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})

		It("returns repository failures", func() {
			repo.EXPECT().
				CreateDraft(gomock.Any(), gomock.Any()).
				Return(nil, errors.New("boom"))

			result, err := svc.CreateDraft(ctx, "freelancer-1")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError("boom"))
		})
	})

	Describe("UpdateBasicInfo", func() {
		It("validates ownership before the rest of the payload", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "other"}, nil)

			result, err := svc.UpdateBasicInfo(ctx, "gig-1", "freelancer-1", domain.UpdateBasicInfoParams{})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidFreelancerID))
		})

		DescribeTable("validates the update payload",
			func(params domain.UpdateBasicInfoParams, expected error) {
				repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

				result, err := svc.UpdateBasicInfo(ctx, "gig-1", "freelancer-1", params)
				Expect(result).To(BeNil())
				Expect(err).To(MatchError(expected))
			},
			Entry("blank title", domain.UpdateBasicInfoParams{Title: "   ", ShortInfo: "short", Description: "desc", CategoryID: 1, Currency: "usd"}, domain.ErrInvalidTitle),
			Entry("blank description", domain.UpdateBasicInfoParams{Title: "title", ShortInfo: "short", Description: "", CategoryID: 1, Currency: "usd"}, domain.ErrInvalidDescription),
			Entry("invalid category", domain.UpdateBasicInfoParams{Title: "title", ShortInfo: "short", Description: "desc", CategoryID: 0, Currency: "usd"}, domain.ErrInvalidCategoryID),
			Entry("blank currency", domain.UpdateBasicInfoParams{Title: "title", ShortInfo: "short", Description: "desc", CategoryID: 1, Currency: "   "}, domain.ErrInvalidCurrency),
		)

		It("rejects an empty slug returned by the slugger", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			slugger.EXPECT().Generate("Title", "gig-1").Return("")

			result, err := svc.UpdateBasicInfo(ctx, "gig-1", "freelancer-1", domain.UpdateBasicInfoParams{
				Title:       " Title ",
				ShortInfo:   " short ",
				Description: "desc",
				CategoryID:  1,
				Currency:    "usd",
			})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidTitle))
		})

		It("updates the repository with normalized values", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			slugger.EXPECT().Generate("My Title", "gig-1").Return("my-title-gig-1")
			repo.EXPECT().
				UpdateBasicInfo(gomock.Any(), "gig-1", domain.UpdateBasicInfoParams{
					Title:       "My Title",
					ShortInfo:   "short",
					Slug:        "my-title-gig-1",
					Description: "desc",
					CategoryID:  1001,
					Currency:    "usd",
				}).
				Return(&domain.Gig{ID: "gig-1"}, nil)
			broker.EXPECT().Publish(gomock.Any(), gigProjectionRequestedSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)

			result, err := svc.UpdateBasicInfo(ctx, "gig-1", "freelancer-1", domain.UpdateBasicInfoParams{
				Title:       " My Title ",
				ShortInfo:   " short ",
				Description: " desc ",
				CategoryID:  1001,
				Currency:    " usd ",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})

		It("returns repository failures", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			slugger.EXPECT().Generate("Title", "gig-1").Return("title-gig-1")
			repo.EXPECT().
				UpdateBasicInfo(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errors.New("update failed"))

			result, err := svc.UpdateBasicInfo(ctx, "gig-1", "freelancer-1", domain.UpdateBasicInfoParams{
				Title:       "Title",
				ShortInfo:   "short",
				Description: "desc",
				CategoryID:  1,
				Currency:    "usd",
			})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError("update failed"))
		})
	})

	Describe("ReplacePackages", func() {
		It("validates ownership and package payload", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

			result, err := svc.ReplacePackages(ctx, "gig-1", "freelancer-1", domain.ReplacePackagesParams{
				Packages: []domain.GigPackage{{Tier: "   "}},
			})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidPackageTier))
		})

		DescribeTable("rejects invalid package shapes",
			func(packages []domain.GigPackage, expected error) {
				repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

				result, err := svc.ReplacePackages(ctx, "gig-1", "freelancer-1", domain.ReplacePackagesParams{Packages: packages})
				Expect(result).To(BeNil())
				Expect(err).To(MatchError(expected))
			},
			Entry("wrong count", []domain.GigPackage{}, domain.ErrInvalidPackageCount),
			Entry("wrong description", []domain.GigPackage{{Tier: domain.TierBasic, Description: "   ", DeliveryDays: 1, PriceCents: 1}}, domain.ErrInvalidPackageDescription),
			Entry("wrong delivery", []domain.GigPackage{{Tier: domain.TierBasic, Description: "desc", DeliveryDays: 0, PriceCents: 1}}, domain.ErrInvalidPackageDeliveryDays),
			Entry("wrong price", []domain.GigPackage{{Tier: domain.TierBasic, Description: "desc", DeliveryDays: 1, PriceCents: 0}}, domain.ErrInvalidPackagePriceCents),
		)

		It("normalizes and replaces packages", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			repo.EXPECT().
				ReplacePackages(gomock.Any(), "gig-1", domain.ReplacePackagesParams{
					Packages: []domain.GigPackage{
						{ID: "p1", GigID: "g1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1},
						{ID: "p2", GigID: "g1", Tier: domain.TierStandard, Description: "standard", DeliveryDays: 2, PriceCents: 200, SortOrder: 2},
						{ID: "p3", GigID: "g1", Tier: domain.TierPremium, Description: "premium", DeliveryDays: 3, PriceCents: 300, SortOrder: 3},
					},
				}).
				Return(&domain.Gig{ID: "gig-1"}, nil)
			broker.EXPECT().Publish(gomock.Any(), gigProjectionRequestedSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)

			result, err := svc.ReplacePackages(ctx, "gig-1", "freelancer-1", domain.ReplacePackagesParams{
				Packages: []domain.GigPackage{
					{ID: " p1 ", GigID: " g1 ", Tier: " basic ", Description: " basic ", DeliveryDays: 1, PriceCents: 100},
					{ID: " p2 ", GigID: " g1 ", Tier: " standard ", Description: " standard ", DeliveryDays: 2, PriceCents: 200},
					{ID: " p3 ", GigID: " g1 ", Tier: " premium ", Description: " premium ", DeliveryDays: 3, PriceCents: 300},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})
	})

	Describe("ReplaceQuestions", func() {
		It("rejects blank content", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

			result, err := svc.ReplaceQuestions(ctx, "gig-1", "freelancer-1", domain.ReplaceQuestionsParams{
				Questions: []domain.GigQuestion{{Content: "   "}},
			})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidQuestionContent))
		})

		It("replaces the questions", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			repo.EXPECT().
				ReplaceQuestions(gomock.Any(), "gig-1", domain.ReplaceQuestionsParams{
					Questions: []domain.GigQuestion{{Content: "question one"}, {Content: "question two"}},
				}).
				Return(&domain.Gig{ID: "gig-1"}, nil)
			broker.EXPECT().Publish(gomock.Any(), gigProjectionRequestedSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)

			result, err := svc.ReplaceQuestions(ctx, "gig-1", "freelancer-1", domain.ReplaceQuestionsParams{
				Questions: []domain.GigQuestion{{Content: "question one"}, {Content: "question two"}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})
	})

	Describe("ReplaceMedia", func() {
		It("validates the upload payload", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

			result, err := svc.ReplaceMedia(ctx, "gig-1", "freelancer-1", domain.ReplaceMediaUploadParams{})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrInvalidMediaUpload))
		})

		It("compensates uploaded files when persistence fails", func() {
			gig := &domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(gig, nil)
			files.EXPECT().
				UploadFiles(gomock.Any(), "freelancer-1", "gigs/gig-1/media", []domain.MediaUpload{
					{Filename: " cover.jpg ", ContentType: " image/jpeg ", Data: []byte("cover")},
					{Filename: " one.jpg ", ContentType: " image/jpeg ", Data: []byte("one")},
				}).
				Return([]string{"cover-file", "gallery-file"}, nil)
			repo.EXPECT().
				ReplaceMedia(gomock.Any(), "gig-1", domain.ReplaceMediaParams{
					PictureFileID: "cover-file",
					Media:         []domain.GigMedia{{GigID: "gig-1", FileID: "gallery-file", SortOrder: 1}},
				}).
				Return(nil, errors.New("persist failed"))
			files.EXPECT().DeleteFile(gomock.Any(), "gallery-file").Return(nil)
			files.EXPECT().DeleteFile(gomock.Any(), "cover-file").Return(nil)

			result, err := svc.ReplaceMedia(ctx, "gig-1", "freelancer-1", domain.ReplaceMediaUploadParams{
				Files: []domain.MediaUpload{
					{Filename: " cover.jpg ", ContentType: " image/jpeg ", Data: []byte("cover")},
					{Filename: " one.jpg ", ContentType: " image/jpeg ", Data: []byte("one")},
				},
			})
			Expect(result).To(BeNil())
			Expect(err).To(MatchError("persist failed"))
		})

		It("replaces media successfully", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)
			files.EXPECT().
				UploadFiles(gomock.Any(), "freelancer-1", "gigs/gig-1/media", gomock.Any()).
				Return([]string{"cover-file", "gallery-file"}, nil)
			repo.EXPECT().
				ReplaceMedia(gomock.Any(), "gig-1", domain.ReplaceMediaParams{
					PictureFileID: "cover-file",
					Media:         []domain.GigMedia{{GigID: "gig-1", FileID: "gallery-file", SortOrder: 1}},
				}).
				Return(&domain.Gig{ID: "gig-1"}, nil)
			broker.EXPECT().Publish(gomock.Any(), gigProjectionRequestedSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)

			result, err := svc.ReplaceMedia(ctx, "gig-1", "freelancer-1", domain.ReplaceMediaUploadParams{
				Files: []domain.MediaUpload{
					{Filename: "cover.jpg", ContentType: "image/jpeg", Data: []byte("cover")},
					{Filename: "one.jpg", ContentType: "image/jpeg", Data: []byte("one")},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})
	})

	Describe("AppendPreviewGig", func() {
		It("rebuilds the preview windows from source when window zero is missing", func() {
			gig := &domain.Gig{
				ID:             "gig-1",
				FreelancerID:   "user-1",
				SellerUsername: "alex1",
				Slug:           "gig-1",
				Title:          "First gig",
				ShortInfo:      "short",
				Status:         domain.StatusPublished,
				CreatedAt:      time.Unix(0, 0).UTC(),
			}

			readRepo.EXPECT().
				ListPreviewWindow(gomock.Any(), "user-1", 0).
				Return(nil, domain.ErrGigNotFound)
			readRepo.EXPECT().
				GetUserLookup(gomock.Any(), "alex1").
				Return("user-1", nil)
			readRepo.EXPECT().
				ListPreviewWindow(gomock.Any(), "user-1", 0).
				Return(nil, domain.ErrGigNotFound)
			repo.EXPECT().
				ListPublishedBySellerUsername(gomock.Any(), domain.ListPreviewGigsQuery{SellerUsername: "alex1", Limit: 0}).
				Return([]*domain.Gig{gig}, nil)
			readRepo.EXPECT().
				SetPopularitySnapshot(gomock.Any(), gomock.Any()).
				Return(nil)
			readRepo.EXPECT().
				ListPopularitySnapshotsByGigIDs(gomock.Any(), []string{"gig-1"}).
				Return(map[string]*domain.GigPopularitySnapshot{}, nil)
			readRepo.EXPECT().
				UpsertPreviewWindow(gomock.Any(), "user-1", 0, gomock.Any(), false, time.Duration(0)).
				Return(nil)
			readRepo.EXPECT().
				AppendPreviewGig(gomock.Any(), "user-1", gomock.Any(), 2, time.Duration(0)).
				Return(nil)

			Expect(svc.AppendPreviewGig(ctx, gig)).To(Succeed())
		})
	})

	Describe("GetByID", func() {
		It("delegates to the repository when the gig is owned", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1"}, nil)

			result, err := svc.GetByID(ctx, "gig-1", "freelancer-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})
	})

	Describe("Publish", func() {
		It("rejects incomplete drafts", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1", Status: domain.StatusDraft}, nil)

			result, err := svc.Publish(ctx, "gig-1", "freelancer-1", "alex")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrGigDraftIncomplete))
		})

		It("rejects already published gigs", func() {
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1", Status: domain.StatusPublished, BasicInfoCompleted: true, PackagesCompleted: true, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}}, nil)

			result, err := svc.Publish(ctx, "gig-1", "freelancer-1", "alex")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrGigAlreadyPublished))
		})

		It("rejects publish when connect onboarding is incomplete", func() {
			connect.EXPECT().GetConnectStatus(gomock.Any(), "freelancer-1").
				Return(&ConnectStatusResult{UserID: "freelancer-1", Status: "pending"}, nil)
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(&domain.Gig{
				ID:                    "gig-1",
				FreelancerID:          "freelancer-1",
				Status:                domain.StatusDraft,
				BasicInfoCompleted:    true,
				PackagesCompleted:     true,
				RequirementsCompleted: true,
				MediaCompleted:        true,
				Packages: []domain.GigPackage{
					{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1},
				},
			}, nil)

			result, err := svc.Publish(ctx, "gig-1", "freelancer-1", "alex")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(domain.ErrConnectOnboardingIncomplete))
		})

		It("publishes the gig and emits the event", func() {
			gig := &domain.Gig{
				ID:                    "gig-1",
				FreelancerID:          "freelancer-1",
				Slug:                  "gig-1",
				Title:                 "Gig One",
				Description:           "desc",
				CategoryID:            1001,
				Currency:              "usd",
				Status:                domain.StatusDraft,
				BasicInfoCompleted:    true,
				PackagesCompleted:     true,
				RequirementsCompleted: true,
				MediaCompleted:        true,
				PictureFileID:         "cover-file",
				Packages: []domain.GigPackage{
					{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100},
				},
				Questions: []domain.GigQuestion{
					{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1},
				},
				Media: []domain.GigMedia{
					{GigID: "gig-1", FileID: "gallery-file", SortOrder: 1},
				},
			}

			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(gig, nil)
			connect.EXPECT().GetConnectStatus(gomock.Any(), "freelancer-1").
				Return(&ConnectStatusResult{UserID: "freelancer-1", Status: "completed"}, nil)
			repo.EXPECT().Publish(gomock.Any(), "gig-1").Return(gig, nil)
			repo.EXPECT().UpdateSellerUsername(gomock.Any(), "gig-1", "alex").Return(&domain.Gig{
				ID:                    "gig-1",
				FreelancerID:          "freelancer-1",
				SellerUsername:        "alex",
				Slug:                  "gig-1",
				Title:                 "Gig One",
				Description:           "desc",
				CategoryID:            1001,
				Currency:              "usd",
				Status:                domain.StatusDraft,
				BasicInfoCompleted:    true,
				PackagesCompleted:     true,
				RequirementsCompleted: true,
				MediaCompleted:        true,
				PictureFileID:         "cover-file",
				Packages:              gig.Packages,
				Questions:             gig.Questions,
				Media:                 gig.Media,
			}, nil)
			broker.EXPECT().Publish(gomock.Any(), gigPublishedSubject, gomock.AssignableToTypeOf([]byte{})).
				DoAndReturn(func(_ context.Context, subject string, payload []byte) error {
					Expect(subject).To(Equal(gigPublishedSubject))
					parsed, err := NewGigEventMapper().FromPublishedPayload(payload)
					Expect(err).NotTo(HaveOccurred())
					Expect(parsed.ID).To(Equal("gig-1"))
					Expect(parsed.Media[0].FileID).To(Equal("gallery-file"))
					return nil
				})
			broker.EXPECT().Publish(gomock.Any(), gigProjectionRequestedSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)
			broker.EXPECT().Publish(gomock.Any(), gigPreviewProjectionSubject, gomock.AssignableToTypeOf([]byte{})).Return(nil)

			result, err := svc.Publish(ctx, "gig-1", "freelancer-1", "alex")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Status).To(Equal(domain.StatusDraft))
		})

		It("returns broker failures", func() {
			gig := &domain.Gig{
				ID:                    "gig-1",
				FreelancerID:          "freelancer-1",
				Slug:                  "gig-1",
				Title:                 "Gig One",
				Description:           "desc",
				CategoryID:            1001,
				Currency:              "usd",
				Status:                domain.StatusDraft,
				BasicInfoCompleted:    true,
				PackagesCompleted:     true,
				RequirementsCompleted: true,
				MediaCompleted:        true,
				Packages: []domain.GigPackage{
					{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100},
				},
			}

			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(gig, nil)
			connect.EXPECT().GetConnectStatus(gomock.Any(), "freelancer-1").
				Return(&ConnectStatusResult{UserID: "freelancer-1", Status: "completed"}, nil)
			repo.EXPECT().Publish(gomock.Any(), "gig-1").Return(gig, nil)
			repo.EXPECT().UpdateSellerUsername(gomock.Any(), "gig-1", "alex").Return(gig, nil)
			broker.EXPECT().Publish(gomock.Any(), gigPublishedSubject, gomock.Any()).Return(errors.New("broker failed"))

			result, err := svc.Publish(ctx, "gig-1", "freelancer-1", "alex")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError("broker failed"))
		})
	})

	Describe("Project", func() {
		It("loads all public urls in a single batch", func() {
			gig := &domain.Gig{
				ID:                    "gig-1",
				PictureFileID:         "cover-file",
				Media:                 []domain.GigMedia{{GigID: "gig-1", FileID: "gallery-1", SortOrder: 1}, {GigID: "gig-1", FileID: "cover-file", SortOrder: 2}},
				PictureURL:            "",
				MediaCompleted:        true,
				RequirementsCompleted: true,
				PackagesCompleted:     true,
				BasicInfoCompleted:    true,
			}

			files.EXPECT().
				GetFileURLs(gomock.Any(), []string{"cover-file", "gallery-1"}).
				Return(map[string]string{
					"cover-file": "https://public.local/cover-file",
					"gallery-1":  "https://public.local/gallery-1",
				}, nil)

			projected, err := svc.Project(ctx, gig)
			Expect(err).NotTo(HaveOccurred())
			Expect(projected.PictureURL).To(Equal("https://public.local/cover-file"))
			Expect(projected.Media).To(HaveLen(2))
			Expect(projected.Media[0].URL).To(Equal("https://public.local/gallery-1"))
			Expect(projected.Media[1].URL).To(Equal("https://public.local/cover-file"))
		})

		It("ignores file-service failures when projecting public urls", func() {
			gig := &domain.Gig{
				ID:            "gig-1",
				PictureFileID: "cover-file",
				Media:         []domain.GigMedia{{GigID: "gig-1", FileID: "gallery-1", SortOrder: 1}},
			}

			files.EXPECT().
				GetFileURLs(gomock.Any(), []string{"cover-file", "gallery-1"}).
				Return(nil, errors.New("file-service down"))

			projected, err := svc.Project(ctx, gig)
			Expect(err).NotTo(HaveOccurred())
			Expect(projected.PictureURL).To(BeEmpty())
			Expect(projected.Media).To(HaveLen(1))
			Expect(projected.Media[0].URL).To(BeEmpty())
		})
	})

	Describe("GetPublicByID", func() {
		It("returns cached gigs and emits a gig.viewed event", func() {
			gig := &domain.Gig{
				ID:             "gig-1",
				FreelancerID:   "freelancer-1",
				SellerUsername: "alex1",
				Slug:           "gig-1",
			}

			readRepo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(gig, nil)
			broker.EXPECT().Publish(gomock.Any(), gigViewedSubject, gomock.AssignableToTypeOf([]byte{})).
				DoAndReturn(func(_ context.Context, subject string, payload []byte) error {
					Expect(subject).To(Equal(gigViewedSubject))
					var event GigViewedEvent
					Expect(json.Unmarshal(payload, &event)).To(Succeed())
					Expect(event.GigID).To(Equal("gig-1"))
					Expect(event.FreelancerID).To(Equal("freelancer-1"))
					Expect(event.SellerUsername).To(Equal("alex1"))
					Expect(event.Slug).To(Equal("gig-1"))
					Expect(event.ViewedAt).NotTo(BeZero())
					return nil
				})

			result, err := svc.GetPublicByID(ctx, "gig-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(gig))
		})

		It("loads from the write repository, projects, caches, and emits a gig.viewed event", func() {
			gig := &domain.Gig{
				ID:             "gig-1",
				FreelancerID:   "freelancer-1",
				SellerUsername: "alex1",
				Slug:           "gig-1",
				Status:         domain.StatusPublished,
			}

			readRepo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(nil, domain.ErrGigNotFound)
			repo.EXPECT().GetByID(gomock.Any(), "gig-1").Return(gig, nil)
			readRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
			broker.EXPECT().Publish(gomock.Any(), gigViewedSubject, gomock.Any()).Return(nil)

			result, err := svc.GetPublicByID(ctx, "gig-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ID).To(Equal("gig-1"))
		})
	})

	Describe("helper functions", func() {
		It("builds media entries with sort order", func() {
			media := buildGigMedia("gig-1", []string{"file-1", "file-2"})
			Expect(media).To(Equal([]domain.GigMedia{
				{GigID: "gig-1", FileID: "file-1", SortOrder: 1},
				{GigID: "gig-1", FileID: "file-2", SortOrder: 2},
			}))
		})

		It("validates media uploads", func() {
			Expect(validateMediaUploads(nil)).To(MatchError(domain.ErrInvalidMediaUpload))
			Expect(validateMediaUploads([]domain.MediaUpload{{Filename: "  ", ContentType: "image/jpeg", Data: []byte("x")}})).To(MatchError(domain.ErrInvalidMediaUpload))
			Expect(validateMediaUploads([]domain.MediaUpload{{Filename: "a", ContentType: "   ", Data: []byte("x")}})).To(MatchError(domain.ErrInvalidMediaUpload))
			Expect(validateMediaUploads([]domain.MediaUpload{{Filename: "a", ContentType: "image/jpeg", Data: nil}})).To(MatchError(domain.ErrInvalidMediaUpload))
			Expect(validateMediaUploads([]domain.MediaUpload{{Filename: "a", ContentType: "image/jpeg", Data: []byte("x")}})).To(Succeed())
		})

		It("validates publish readiness", func() {
			Expect(validatePublishReady(&domain.Gig{Status: domain.StatusPublished, BasicInfoCompleted: true, PackagesCompleted: true, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}})).To(MatchError(domain.ErrGigAlreadyPublished))
			Expect(validatePublishReady(&domain.Gig{Status: domain.StatusDraft, PackagesCompleted: true, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}})).To(MatchError(domain.ErrGigDraftIncomplete))
			Expect(validatePublishReady(&domain.Gig{Status: domain.StatusDraft, BasicInfoCompleted: true, PackagesCompleted: false, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}})).To(MatchError(domain.ErrGigDraftIncomplete))
			Expect(validatePublishReady(&domain.Gig{Status: domain.StatusDraft, BasicInfoCompleted: true, PackagesCompleted: true, Packages: []domain.GigPackage{{Tier: "wrong", Description: "basic", DeliveryDays: 1, PriceCents: 1}}})).To(MatchError(domain.ErrInvalidPackageTier))
			Expect(validatePublishReady(&domain.Gig{Status: domain.StatusDraft, BasicInfoCompleted: true, PackagesCompleted: true, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}})).To(Succeed())
		})

		It("trims the gig media prefix", func() {
			Expect(gigMediaPrefix(" gig-1 ")).To(Equal("gigs/gig-1/media"))
		})
	})

	Describe("slugger", func() {
		It("generates slugs using the production implementation", func() {
			Expect(NewSlugger().Generate(" Gig Title 123 ", "gig-1")).To(Equal("gig-title-123-gig-1"))
		})
	})

	Describe("gig event mapper", func() {
		It("round-trips published payloads", func() {
			when := time.Unix(1735689600, 123).UTC()
			gig := &domain.Gig{
				ID:                    "gig-1",
				FreelancerID:          "freelancer-1",
				Slug:                  "gig-1",
				Title:                 "Gig One",
				Description:           "desc",
				CategoryID:            1001,
				Currency:              "usd",
				Status:                domain.StatusDraft,
				BasicInfoCompleted:    true,
				PackagesCompleted:     true,
				RequirementsCompleted: true,
				MediaCompleted:        true,
				PictureFileID:         "cover-file",
				PublishedAt:           &when,
				CreatedAt:             when,
				UpdatedAt:             when,
				Packages: []domain.GigPackage{
					{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100},
				},
				Questions: []domain.GigQuestion{
					{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1},
				},
				Media: []domain.GigMedia{
					{GigID: "gig-1", FileID: "gallery-file", SortOrder: 1},
				},
			}

			payload, err := NewGigEventMapper().ToPublishedPayload(gig)
			Expect(err).NotTo(HaveOccurred())

			parsed, err := NewGigEventMapper().FromPublishedPayload(payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed).To(Equal(gig))
		})

		It("builds viewed payloads", func() {
			gig := &domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1", SellerUsername: "alex1", Slug: "gig-1"}

			payload, err := NewGigEventMapper().ToViewedPayload(gig)
			Expect(err).NotTo(HaveOccurred())

			var event GigViewedEvent
			Expect(json.Unmarshal(payload, &event)).To(Succeed())
			Expect(event.GigID).To(Equal("gig-1"))
			Expect(event.FreelancerID).To(Equal("freelancer-1"))
			Expect(event.SellerUsername).To(Equal("alex1"))
			Expect(event.Slug).To(Equal("gig-1"))
			Expect(event.ViewedAt).NotTo(BeZero())
		})

		It("returns JSON decoding failures", func() {
			parsed, err := NewGigEventMapper().FromPublishedPayload([]byte("not-json"))
			Expect(parsed).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
})
