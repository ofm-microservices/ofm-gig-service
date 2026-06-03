package grpc

import (
	"context"
	app "gig-service/internal/application"
	userv1 "github.com/ofm-microservices/ofm-common/proto/user/v1"
)

// UserPreviewClient exposes the user-service lookup used by gig-service
// bootstrap to backfill seller usernames.
type UserPreviewClient interface {
	GetUserPreviewByIDNoCache(ctx context.Context, userID string) (*app.UserPreview, error)
	Close() error
}

// UserMapper translates between gig-service inputs and the shared user gRPC
// contract.
type UserMapper interface {
	ToGetUserPreviewByIDNoCacheRequest(userID string) *userv1.GetUserPreviewByIDNoCacheRequest
	ToGetUserPreviewByIDNoCacheResponse(res *userv1.GetUserPreviewByIDNoCacheResponse) *app.UserPreview
	ToError(err error) error
}
