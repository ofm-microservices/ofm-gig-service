package grpc

import (
	"context"

	app "gig-service/internal/application"
	paymentconnectv1 "github.com/ofm-microservices/ofm-common/proto/paymentconnect/v1"
)

// PaymentService exposes the payment-service gRPC client used by gig-service
// to check Connect onboarding state before publication.
type PaymentService interface {
	GetConnectStatus(ctx context.Context, userID string) (*app.ConnectStatusResult, error)
	Close() error
}

// PaymentMapper translates between gig-service inputs and the shared
// paymentconnect gRPC contract.
type PaymentMapper interface {
	ToGetConnectStatusRequest(userID string) *paymentconnectv1.GetConnectStatusRequest
	ToGetConnectStatusResponse(res *paymentconnectv1.GetConnectStatusResponse) *app.ConnectStatusResult
	ToError(err error) error
}
