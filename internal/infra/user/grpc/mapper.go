package grpc

import (
	"fmt"
	app "gig-service/internal/application"
	userv1 "github.com/ofm-microservices/ofm-common/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userMapper struct{}

func newUserMapper() UserMapper { return &userMapper{} }

func (m *userMapper) ToGetUserPreviewByIDNoCacheRequest(userID string) *userv1.GetUserPreviewByIDNoCacheRequest {
	return &userv1.GetUserPreviewByIDNoCacheRequest{UserId: userID}
}

func (m *userMapper) ToGetUserPreviewByIDNoCacheResponse(res *userv1.GetUserPreviewByIDNoCacheResponse) *app.UserPreview {
	if res == nil || res.GetUser() == nil {
		return nil
	}
	user := res.GetUser()
	return &app.UserPreview{
		UserID:      user.GetUserId(),
		Username:    user.GetUsername(),
		DisplayName: user.GetDisplayName(),
		AvatarID:    user.GetAvatarId(),
		AvatarURL:   user.GetAvatarUrl(),
	}
}

func (m *userMapper) ToError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("user not found")
	default:
		return err
	}
}
