package grpc

import (
	"context"

	app "gig-service/internal/application"
	orderwritev1 "github.com/ofm-microservices/ofm-common/proto/orderwrite/v1"
)

// OrderCountService exposes the order-service count lookup used by gig-service
// to backfill gig preview aggregates.
type OrderCountService interface {
	GetOrderCountByGigID(ctx context.Context, gigID string) (*app.OrderCountResult, error)
	Close() error
}

// OrderCountMapper translates between gig-service inputs and the shared
// orderwrite gRPC contract.
type OrderCountMapper interface {
	ToGetOrderCountByGigIDRequest(gigID string) *orderwritev1.GetOrderCountByGigIDRequest
	ToGetOrderCountByGigIDResponse(res *orderwritev1.GetOrderCountByGigIDResponse) *app.OrderCountResult
}
