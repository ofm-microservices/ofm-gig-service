package grpc

import (
	"errors"
	"strings"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	paymentconnectv1 "github.com/ofm-microservices/ofm-common/proto/paymentconnect/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentMapper struct{}

func newPaymentMapper() PaymentMapper {
	return &paymentMapper{}
}

func (m *paymentMapper) ToGetConnectStatusRequest(userID string) *paymentconnectv1.GetConnectStatusRequest {
	return &paymentconnectv1.GetConnectStatusRequest{UserId: userID}
}

func (m *paymentMapper) ToGetConnectStatusResponse(res *paymentconnectv1.GetConnectStatusResponse) *app.ConnectStatusResult {
	if res == nil {
		return nil
	}

	return &app.ConnectStatusResult{
		UserID:          res.GetUserId(),
		Status:          res.GetStatus(),
		StripeAccountID: res.GetStripeAccountId(),
		DisabledReason:  res.GetDisabledReason(),
		OccurredAt:      res.GetOccurredAt(),
	}
}

func (m *paymentMapper) ToError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound, codes.FailedPrecondition:
		return domain.ErrConnectOnboardingIncomplete
	case codes.Unknown:
		if strings.Contains(strings.ToLower(st.Message()), "payment connect account not found") {
			return domain.ErrConnectOnboardingIncomplete
		}
		return domain.ErrConnectOnboardingIncomplete
	default:
		if errors.Is(err, domain.ErrConnectOnboardingIncomplete) {
			return err
		}
		return err
	}
}
