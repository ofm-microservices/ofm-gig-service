package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

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
		mapper.EXPECT().ToViewedPayload(gomock.Any()).Return(nil, nil).AnyTimes()
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

	It("stores freelancer preview windows as a zset plus gig payload keys", func() {
		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, app.NewGigEventMapper(), lg)
		Expect(err).NotTo(HaveOccurred())

		gigs := []*domain.GigPreview{
			{
				ID:                "gig-2",
				FreelancerID:      "user-1",
				SellerUsername:    "alex1",
				Slug:              "gig-2",
				Title:             "Second",
				ShortInfo:         "short-2",
				MinimumPriceCents: 2500,
				PictureURL:        "https://cdn.example/gig-2.jpg",
				PopularityScore:   250,
			},
			{
				ID:                "gig-1",
				FreelancerID:      "user-1",
				SellerUsername:    "alex1",
				Slug:              "gig-1",
				Title:             "First",
				ShortInfo:         "short-1",
				MinimumPriceCents: 1000,
				PictureURL:        "https://cdn.example/gig-1.jpg",
				PopularityScore:   300,
			},
		}

		Expect(repo.UpsertPreviewWindow(context.Background(), "user-1", 0, gigs, true, 15*time.Minute)).To(Succeed())

		zcard, err := rdb.ZCard(context.Background(), GigPreviewWindowKey("user-1", 0)).Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(zcard).To(Equal(int64(2)))
		payload1, err := rdb.Get(context.Background(), GigPreviewPayloadKey("gig-1")).Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(payload1).To(ContainSubstring(`"id":"gig-1"`))
		Expect(payload1).To(ContainSubstring(`"seller_username":"alex1"`))
		Expect(payload1).NotTo(ContainSubstring("SellerUsername"))

		window, err := repo.ListPreviewWindow(context.Background(), "user-1", 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(window).NotTo(BeNil())
		Expect(window.HasMore).To(BeFalse())
		Expect(window.Gigs).To(HaveLen(2))
		Expect(window.Gigs[0].ID).To(Equal("gig-1"))
		Expect(window.Gigs[0].PopularityScore).To(Equal(int64(300)))
		Expect(window.Gigs[1].ID).To(Equal("gig-2"))
	})

	It("appends published gigs to the tail window without rebuilding the cache", func() {
		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, app.NewGigEventMapper(), lg)
		Expect(err).NotTo(HaveOccurred())

		first := &domain.GigPreview{
			ID:                "gig-1",
			FreelancerID:      "user-1",
			SellerUsername:    "alex1",
			Slug:              "gig-1",
			Title:             "First",
			ShortInfo:         "short-1",
			MinimumPriceCents: 1000,
			PictureURL:        "https://cdn.example/gig-1.jpg",
			PopularityScore:   0,
		}
		second := &domain.GigPreview{
			ID:                "gig-2",
			FreelancerID:      "user-1",
			SellerUsername:    "alex1",
			Slug:              "gig-2",
			Title:             "Second",
			ShortInfo:         "short-2",
			MinimumPriceCents: 2000,
			PictureURL:        "https://cdn.example/gig-2.jpg",
			PopularityScore:   0,
		}
		third := &domain.GigPreview{
			ID:                "gig-3",
			FreelancerID:      "user-1",
			SellerUsername:    "alex1",
			Slug:              "gig-3",
			Title:             "Third",
			ShortInfo:         "short-3",
			MinimumPriceCents: 3000,
			PictureURL:        "https://cdn.example/gig-3.jpg",
			PopularityScore:   0,
		}

		Expect(repo.AppendPreviewGig(context.Background(), "user-1", first, 2, 15*time.Minute)).To(Succeed())
		Expect(repo.AppendPreviewGig(context.Background(), "user-1", second, 2, 15*time.Minute)).To(Succeed())
		Expect(repo.AppendPreviewGig(context.Background(), "user-1", third, 2, 15*time.Minute)).To(Succeed())
		Expect(repo.AppendPreviewGig(context.Background(), "user-1", third, 2, 15*time.Minute)).To(Succeed())

		window0, err := repo.ListPreviewWindow(context.Background(), "user-1", 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(window0.Gigs).To(HaveLen(2))
		Expect(window0.Gigs[0].ID).To(Equal("gig-2"))
		Expect(window0.Gigs[1].ID).To(Equal("gig-1"))

		window1, err := repo.ListPreviewWindow(context.Background(), "user-1", 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(window1.Gigs).To(HaveLen(1))
		Expect(window1.Gigs[0].ID).To(Equal("gig-3"))

		ttl0, err := rdb.TTL(context.Background(), GigPreviewWindowKey("user-1", 0)).Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(ttl0).To(Equal(time.Duration(-1)))
	})
})
