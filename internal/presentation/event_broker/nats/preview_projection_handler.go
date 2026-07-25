package nats

import (
	"context"
)

func (s *gigPreviewProjectionSubscriber) handleGigPreviewProjectionRequestedEvent(ctx context.Context, _ string, payload []byte) error {
	gig, err := s.mapr.FromPublishedPayload(payload)
	if err != nil {
		return err
	}
	if gig == nil {
		return ErrInvalidGigPayload
	}
	if gig.ID == "" {
		return ErrInvalidGigPayload
	}

	projected, err := s.svc.Project(ctx, gig)
	if err != nil {
		return err
	}
	if projected == nil {
		return ErrInvalidGigPayload
	}
	return s.svc.AppendPreviewGig(ctx, projected)
}
