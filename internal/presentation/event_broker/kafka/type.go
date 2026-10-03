package kafka

import (
	"context"
	"gig-service/internal/domain"
)

// GigProjectionSubscriber consumes gig lifecycle events and writes the
// service-owned Redis read model.
type GigProjectionSubscriber interface {
	Subscribe(context.Context) error
}

// GigPreviewProjectionSubscriber consumes publish-time events and appends
// published gigs to freelancer preview windows.
type GigPreviewProjectionSubscriber interface {
	Subscribe(context.Context) error
}

// GigReviewRatingSubscriber refreshes cached rating fields after review
// aggregates change.
type GigReviewRatingSubscriber interface {
	Subscribe(context.Context) error
}

// GigOrderCountSubscriber refreshes cached order counts after funded orders.
type GigOrderCountSubscriber interface {
	Subscribe(context.Context) error
}

// ProjectionWriter stores projected gig read models.
type ProjectionWriter interface {
	Upsert(context.Context, *domain.Gig) error
	DeleteByID(context.Context, string) error
}
