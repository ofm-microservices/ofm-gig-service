package repository

import (
	"context"
	"encoding/json"
	"errors"

	app "gig-service/internal/application"
	"gig-service/internal/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

var _ = Describe("redis repository", func() {
	var lg logging.Logger

	BeforeEach(func() {
		var err error
		lg, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("validates constructor inputs", func() {
		repo, err := New(nil, app.NewGigEventMapper(), lg)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilRedisClient))

		rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
		repo, err = New(rdb, nil, lg)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilGig))

		repo, err = New(rdb, app.NewGigEventMapper(), nil)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("upserts and deletes the read model in redis", func() {
		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, app.NewGigEventMapper(), lg)
		Expect(err).NotTo(HaveOccurred())

		gig := &domain.Gig{
			ID:            "gig-1",
			FreelancerID:  "freelancer-1",
			Slug:          "gig-1",
			Title:         "Gig One",
			Description:   "desc",
			CategoryID:    1001,
			Currency:      "usd",
			PictureFileID: "cover-file",
			Packages:      []domain.GigPackage{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
			Media:         []domain.GigMedia{{GigID: "gig-1", FileID: "file-1", SortOrder: 1}},
		}

		Expect(repo.Upsert(context.Background(), gig)).To(Succeed())
		raw, err := rdb.Get(context.Background(), GigCacheKey("gig-1")).Result()
		Expect(err).NotTo(HaveOccurred())

		var payload app.GigReadModelEvent
		Expect(json.Unmarshal([]byte(raw), &payload)).To(Succeed())
		Expect(payload.GigID).To(Equal("gig-1"))
		Expect(payload.Media[0].ID).To(Equal("file-1"))

		loaded, err := repo.GetByID(context.Background(), "gig-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.ID).To(Equal("gig-1"))
		Expect(loaded.Media[0].FileID).To(Equal("file-1"))

		Expect(repo.DeleteByID(context.Background(), "gig-1")).To(Succeed())
		_, err = rdb.Get(context.Background(), GigCacheKey("gig-1")).Result()
		Expect(err).To(HaveOccurred())
	})

	It("returns cache errors and mapper failures", func() {
		ctrl := gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		mapper := NewMockGigEventMapper(ctrl)
		mapper.EXPECT().ToReadModelPayload(gomock.Any()).Return(nil, errors.New("marshal failed"))

		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, mapper, lg)
		Expect(err).NotTo(HaveOccurred())

		Expect(repo.Upsert(context.Background(), &domain.Gig{ID: "gig-1"})).To(MatchError(ContainSubstring("marshal gig cache")))
		Expect(repo.Upsert(context.Background(), nil)).To(MatchError(ErrNilGig))
		_, err = repo.GetByID(context.Background(), "missing")
		Expect(err).To(MatchError(domain.ErrGigNotFound))
		Expect(WrapMarshalGigCacheError(errors.New("boom"))).To(MatchError(ContainSubstring("marshal gig cache")))
		Expect(WrapGetGigCacheError("gig:1", errors.New("boom"))).To(MatchError(ContainSubstring("get gig cache")))
		Expect(WrapUnmarshalGigCacheError(errors.New("boom"))).To(MatchError(ContainSubstring("unmarshal gig cache")))
		Expect(WrapSetGigCacheError("gig:1", errors.New("boom"))).To(MatchError(ContainSubstring("set gig cache")))
		Expect(WrapDeleteGigCacheError("gig:1", errors.New("boom"))).To(MatchError(ContainSubstring("delete gig cache")))
	})
})
