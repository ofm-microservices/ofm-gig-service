package grpc

import (
	app "gig-service/internal/application"
	orderwritev1 "github.com/ofm-microservices/ofm-common/proto/orderwrite/v1"
)

type orderCountMapper struct{}

func newOrderCountMapper() OrderCountMapper { return &orderCountMapper{} }

func (m *orderCountMapper) ToGetOrderCountByGigIDRequest(gigID string) *orderwritev1.GetOrderCountByGigIDRequest {
	return &orderwritev1.GetOrderCountByGigIDRequest{GigId: gigID}
}

func (m *orderCountMapper) ToGetOrderCountByGigIDResponse(res *orderwritev1.GetOrderCountByGigIDResponse) *app.OrderCountResult {
	if res == nil {
		return nil
	}
	return &app.OrderCountResult{
		OrderCount: res.GetOrderCount(),
	}
}
