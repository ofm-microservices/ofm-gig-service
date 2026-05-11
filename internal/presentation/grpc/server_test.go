package grpc

import (
	"context"
	"errors"
	"time"

	"gig-service/config"
	"gig-service/internal/domain"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	gigv1 "github.com/ofm-microservices/ofm-common/proto/gig/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("gig gRPC mapper", func() {
	var lg logging.Logger

	BeforeEach(func() {
		var err error
		lg, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("maps a gig response with timestamps and nested payloads", func() {
		when := time.Unix(1735689600, 123).UTC()
		mapper := newGigMapper(lg)
		resp := mapper.ToGigResponse(&domain.Gig{
			ID:                    "gig-1",
			FreelancerID:          "freelancer-1",
			Slug:                  "gig-1",
			Title:                 "Gig One",
			Description:           "desc",
			CategoryID:            1001,
			Currency:              "usd",
			Status:                domain.StatusPublished,
			BasicInfoCompleted:    true,
			PackagesCompleted:     true,
			RequirementsCompleted: true,
			MediaCompleted:        true,
			PictureFileID:         "cover-file",
			PublishedAt:           &when,
			CreatedAt:             when,
			UpdatedAt:             when,
			Packages:              []domain.GigPackage{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
			Questions:             []domain.GigQuestion{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}},
			Media:                 []domain.GigMedia{{GigID: "gig-1", FileID: "file-1", SortOrder: 1}},
		})

		Expect(resp.GigId).To(Equal("gig-1"))
		Expect(resp.PublishedAt).To(Equal(when.Format(timeFormat)))
		Expect(resp.CreatedAt).To(Equal(when.Format(timeFormat)))
		Expect(resp.Packages[0].Tier).To(Equal(domain.TierBasic))
		Expect(resp.Questions[0].Content).To(Equal("question"))
		Expect(resp.Media[0].FileId).To(Equal("file-1"))
	})

	It("maps nil gig values and request params", func() {
		mapper := newGigMapper(lg)
		Expect(mapper.ToGigResponse(nil)).To(BeNil())
		Expect(mapper.ToCreateDraftResponse(nil).Gig).To(BeNil())

		Expect(mapper.ToUpdateBasicInfoParams(&gigv1.UpdateBasicInfoRequest{
			Title:       "title",
			Description: "desc",
			CategoryId:  1001,
			Currency:    "usd",
		})).To(Equal(domain.UpdateBasicInfoParams{
			Title:       "title",
			Description: "desc",
			CategoryID:  1001,
			Currency:    "usd",
		}))

		Expect(mapper.ToReplacePackagesParams(&gigv1.ReplacePackagesRequest{
			Packages: []*gigv1.GigPackage{{Id: "pkg-1", GigId: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
		})).To(Equal(domain.ReplacePackagesParams{
			Packages: []domain.GigPackage{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
		}))

		Expect(mapper.ToReplaceQuestionsParams(&gigv1.ReplaceQuestionsRequest{
			Questions: []*gigv1.GigQuestion{{Id: "q-1", GigId: "gig-1", Content: "question", SortOrder: 1}},
		})).To(Equal(domain.ReplaceQuestionsParams{
			Questions: []domain.GigQuestion{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}},
		}))

		Expect(mapper.ToReplaceMediaParams(&gigv1.ReplaceMediaRequest{
			Files: []*gigv1.MediaUpload{{Filename: "file", ContentType: "image/jpeg", Data: []byte("x")}},
		})).To(Equal(domain.ReplaceMediaUploadParams{
			Files: []domain.MediaUpload{{Filename: "file", ContentType: "image/jpeg", Data: []byte("x")}},
		}))
	})

	It("maps domain errors to transport status codes", func() {
		mapper := newGigMapper(lg)
		Expect(status.Code(mapper.ToError(domain.ErrInvalidGigID))).To(Equal(codes.InvalidArgument))
		Expect(status.Code(mapper.ToError(domain.ErrGigNotFound))).To(Equal(codes.NotFound))
		Expect(status.Code(mapper.ToError(domain.ErrGigDraftIncomplete))).To(Equal(codes.FailedPrecondition))
		Expect(status.Code(mapper.ToError(errors.New("boom")))).To(Equal(codes.Internal))
		Expect(mapper.ToError(nil)).To(BeNil())
	})
})

var _ = Describe("gig gRPC server", func() {
	var (
		ctrl   *gomock.Controller
		svc    *MockGigService
		logger logging.Logger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		svc = NewMockGigService(ctrl)

		var err error
		logger, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("validates constructor dependencies", func() {
		srv, err := NewServer(nil, config.GRPCConfig{}, logger)
		Expect(srv).To(BeNil())
		Expect(err).To(MatchError(ErrNilGigService))

		srv, err = NewServer(svc, config.GRPCConfig{}, nil)
		Expect(srv).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("starts and shuts down the grpc server", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{Host: "127.0.0.1", Port: 0}, logger)
		Expect(err).NotTo(HaveOccurred())
		impl := srvAny.(*server)

		done := make(chan error, 1)
		go func() {
			done <- impl.Start()
		}()

		Eventually(func() bool { return impl.listener != nil }).Should(BeTrue())

		Expect(impl.Shutdown(context.Background())).To(HaveOccurred())
		Eventually(done).Should(Receive(BeNil()))
	})

	It("returns listen errors from Start", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{Host: "not a host", Port: 12345}, logger)
		Expect(err).NotTo(HaveOccurred())
		impl := srvAny.(*server)

		err = impl.Start()
		Expect(err).To(HaveOccurred())
	})

	It("shuts down cleanly when the listener is absent", func() {
		impl := &server{}
		Expect(impl.Shutdown(context.Background())).To(Succeed())
	})

	DescribeTable("transport methods",
		func(call func(*server) (any, error)) {
			srvAny, err := NewServer(svc, config.GRPCConfig{}, logger)
			Expect(err).NotTo(HaveOccurred())
			impl := srvAny.(*server)

			result, err := call(impl)
			Expect(result).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("CreateDraft", func(impl *server) (any, error) {
			svc.EXPECT().CreateDraft(gomock.Any(), "freelancer-1").Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.CreateDraft(context.Background(), &gigv1.CreateDraftRequest{FreelancerId: "freelancer-1"})
		}),
		Entry("UpdateBasicInfo", func(impl *server) (any, error) {
			svc.EXPECT().UpdateBasicInfo(gomock.Any(), "gig-1", "freelancer-1", gomock.Any()).Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.UpdateBasicInfo(context.Background(), &gigv1.UpdateBasicInfoRequest{GigId: "gig-1", FreelancerId: "freelancer-1", Title: "title", Description: "desc", CategoryId: 1, Currency: "usd"})
		}),
		Entry("ReplacePackages", func(impl *server) (any, error) {
			svc.EXPECT().ReplacePackages(gomock.Any(), "gig-1", "freelancer-1", gomock.Any()).Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.ReplacePackages(context.Background(), &gigv1.ReplacePackagesRequest{GigId: "gig-1", FreelancerId: "freelancer-1", Packages: []*gigv1.GigPackage{{Tier: domain.TierBasic, Description: "desc", DeliveryDays: 1, PriceCents: 1}}})
		}),
		Entry("ReplaceQuestions", func(impl *server) (any, error) {
			svc.EXPECT().ReplaceQuestions(gomock.Any(), "gig-1", "freelancer-1", gomock.Any()).Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.ReplaceQuestions(context.Background(), &gigv1.ReplaceQuestionsRequest{GigId: "gig-1", FreelancerId: "freelancer-1", Questions: []*gigv1.GigQuestion{{Content: "question"}}})
		}),
		Entry("ReplaceMedia", func(impl *server) (any, error) {
			svc.EXPECT().ReplaceMedia(gomock.Any(), "gig-1", "freelancer-1", gomock.Any()).Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.ReplaceMedia(context.Background(), &gigv1.ReplaceMediaRequest{GigId: "gig-1", FreelancerId: "freelancer-1", Files: []*gigv1.MediaUpload{{Filename: "file", ContentType: "image/jpeg", Data: []byte("x")}}})
		}),
		Entry("GetDraft", func(impl *server) (any, error) {
			svc.EXPECT().GetByID(gomock.Any(), "gig-1", "freelancer-1").Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.GetDraft(context.Background(), &gigv1.GetDraftRequest{GigId: "gig-1", FreelancerId: "freelancer-1"})
		}),
		Entry("Publish", func(impl *server) (any, error) {
			svc.EXPECT().Publish(gomock.Any(), "gig-1", "freelancer-1").Return(&domain.Gig{ID: "gig-1"}, nil)
			return impl.Publish(context.Background(), &gigv1.PublishRequest{GigId: "gig-1", FreelancerId: "freelancer-1"})
		}),
	)

	It("maps transport errors from the service", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())
		impl := srvAny.(*server)

		svc.EXPECT().CreateDraft(gomock.Any(), "freelancer-1").Return(nil, domain.ErrInvalidFreelancerID)
		_, err = impl.CreateDraft(context.Background(), &gigv1.CreateDraftRequest{FreelancerId: "freelancer-1"})
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
	})
})
