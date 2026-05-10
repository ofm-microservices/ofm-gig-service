package nats

import (
	"context"
)

func (s *gigProjectionSubscriber) handleGigPublishedEvent(ctx context.Context, _ string, payload []byte) error {
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

	if err := s.writer.Upsert(ctx, gig); err != nil {
		return err
	}

	return nil
}
