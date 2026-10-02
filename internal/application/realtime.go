package service

import (
	"context"
	"encoding/json"

	"gig-service/internal/domain"
	commonrealtime "github.com/ofm-microservices/ofm-common/pkg/realtime"
)

const realtimeSubject = "realtime"

// PublishGigNotification publishes a user-visible gig outcome. Projection and
// recovery events remain on their internal topics and are not exposed here.
func PublishGigNotification(ctx context.Context, broker EventBroker, gig *domain.Gig, eventType string, payload json.RawMessage) error {
	return PublishGigNotificationOutcome(ctx, broker, gig, eventType, commonrealtime.StatusAccepted, "", "", "", nil, payload)
}

// PublishGigNotificationOutcome publishes a terminal or intermediate gig
// outcome while preserving the operation identity from the source event.
func PublishGigNotificationOutcome(ctx context.Context, broker EventBroker, gig *domain.Gig, eventType, status, errorCode, operationID, correlationID string, retryable *bool, payload json.RawMessage) error {
	if gig == nil {
		return domain.ErrGigNotFound
	}
	notification := commonrealtime.WithOutcome(ctx, eventType, "gig", gig.ID, gig.FreelancerID, operationID, correlationID, status, errorCode, retryable, payload)
	body, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	return broker.Publish(ctx, realtimeSubject, body)
}
