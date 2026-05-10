package grpc

import (
	"context"

	"gig-service/internal/domain"
	filev1 "github.com/ofm-microservices/ofm-common/proto/file/v1"
)

// FileService exposes the file-service client used by gig-service.
type FileService interface {
	UploadFiles(ctx context.Context, ownerID, prefix string, files []domain.MediaUpload) ([]string, error)
	DeleteFile(ctx context.Context, fileID string) error
	Close() error
}

// FileMapper translates between gig-service file upload inputs and the
// file-service gRPC contract.
type FileMapper interface {
	ToUploadFilesRequest(ownerID, prefix string, files []domain.MediaUpload) *filev1.UploadFilesRequest
	ToUploadFilesResponse(res *filev1.UploadFilesResponse) []string
	ToDeleteFileRequest(fileID string) *filev1.DeleteFileRequest
	ToError(err error) error
}
