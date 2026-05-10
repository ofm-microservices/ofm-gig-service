package mapper

import (
	"testing"
	"time"

	"gig-service/internal/domain"
	"gig-service/internal/infra/write/yugabyte/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMapper(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gig Yugabyte Mapper Suite")
}

var _ = Describe("gig row mapper", func() {
	It("maps nested storage rows to the domain aggregate", func() {
		when := time.Unix(1735689600, 0).UTC()
		gig := MapGigRowToDomain(
			model.GigRow{
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
			},
			[]model.GigPackageRow{{ID: "pkg-1", GigID: "gig-1", Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 100, SortOrder: 1}},
			[]model.GigQuestionRow{{ID: "q-1", GigID: "gig-1", Content: "question", SortOrder: 1}},
			[]model.GigMediaRow{{ID: "media-1", GigID: "gig-1", FileID: "file-1", SortOrder: 1}},
		)

		Expect(gig.ID).To(Equal("gig-1"))
		Expect(gig.Packages).To(HaveLen(1))
		Expect(gig.Questions).To(HaveLen(1))
		Expect(gig.Media).To(HaveLen(1))
	})
})
