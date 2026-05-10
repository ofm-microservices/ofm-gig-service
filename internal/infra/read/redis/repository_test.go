package repository

import (
	"context"
	"encoding/json"
	"errors"

	app "gig-service/internal/application"
	"gig-service/internal/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("redis repository", func() {
	It("validates constructor inputs", func() {
		repo, err := New(nil, app.NewGigEventMapper())
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilRedisClient))

		rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
		repo, err = New(rdb, nil)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilGig))
	})

	It("upserts and deletes the read model in redis", func() {
		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, app.NewGigEventMapper())
		Expect(err).NotTo(HaveOccurred())

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
			Packages: []domain.GigPackage{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
			Questions: []domain.GigQuestion{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}},
			Media: []domain.GigMedia{{GigID: "gig-1", FileID: "file-1", SortOrder: 1}},
		}

		Expect(repo.Upsert(context.Background(), gig)).To(Succeed())
		raw, err := rdb.Get(context.Background(), GigCacheKey("gig-1")).Result()
		Expect(err).NotTo(HaveOccurred())

		var payload app.GigPublishedEvent
		Expect(json.Unmarshal([]byte(raw), &payload)).To(Succeed())
		Expect(payload.GigID).To(Equal("gig-1"))
		Expect(payload.Media[0].FileID).To(Equal("file-1"))

		Expect(repo.DeleteByID(context.Background(), "gig-1")).To(Succeed())
		_, err = rdb.Get(context.Background(), GigCacheKey("gig-1")).Result()
		Expect(err).To(HaveOccurred())
	})

	It("returns cache errors and mapper failures", func() {
		ctrl := gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		mapper := NewMockGigEventMapper(ctrl)
		mapper.EXPECT().ToPublishedPayload(gomock.Any()).Return(nil, errors.New("marshal failed"))

		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()

		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
		repo, err := New(rdb, mapper)
		Expect(err).NotTo(HaveOccurred())

		Expect(repo.Upsert(context.Background(), &domain.Gig{ID: "gig-1"})).To(MatchError(ContainSubstring("marshal gig cache")))
		Expect(repo.Upsert(context.Background(), nil)).To(MatchError(ErrNilGig))
		Expect(WrapMarshalGigCacheError(errors.New("boom"))).To(MatchError(ContainSubstring("marshal gig cache")))
		Expect(WrapSetGigCacheError("gig:1", errors.New("boom"))).To(MatchError(ContainSubstring("set gig cache")))
		Expect(WrapDeleteGigCacheError("gig:1", errors.New("boom"))).To(MatchError(ContainSubstring("delete gig cache")))
	})
})
