package grpc

import (
	"errors"
	"gig-service/internal/domain"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	filev1 "github.com/ofm-microservices/ofm-common/proto/file/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fileMapper struct {
	log logging.Logger
}

func newFileMapper(log logging.Logger) FileMapper {
	return &fileMapper{log: log}
}

func (m *fileMapper) ToUploadFilesRequest(ownerID, prefix string, files []domain.MediaUpload) *filev1.UploadFilesRequest {
	items := make([]*filev1.UploadFileInput, 0, len(files))
	for _, item := range files {
		items = append(items, &filev1.UploadFileInput{
			Filename:    item.Filename,
			ContentType: item.ContentType,
			Data:        item.Data,
		})
	}

	return &filev1.UploadFilesRequest{
		OwnerId: ownerID,
		Prefix:  prefix,
		Files:   items,
	}
}

func (m *fileMapper) ToUploadFilesResponse(res *filev1.UploadFilesResponse) []string {
	if res == nil || len(res.GetFiles()) == 0 {
		return nil
	}

	ids := make([]string, 0, len(res.GetFiles()))
	for _, item := range res.GetFiles() {
		ids = append(ids, item.GetFileId())
	}

	return ids
}

func (m *fileMapper) ToGetFileURLRequest(fileID string) *filev1.GetFileURLRequest {
	return &filev1.GetFileURLRequest{FileId: fileID}
}

func (m *fileMapper) ToGetFileURLResponse(res *filev1.GetFileURLResponse) string {
	if res == nil {
		return ""
	}
	return res.GetUrl()
}

func (m *fileMapper) ToGetFileURLsRequest(fileIDs []string) *filev1.GetFileURLsRequest {
	return &filev1.GetFileURLsRequest{FileIds: fileIDs}
}

func (m *fileMapper) ToGetFileURLsResponse(res *filev1.GetFileURLsResponse) map[string]string {
	if res == nil || len(res.GetFileUrls()) == 0 {
		return nil
	}

	urls := make(map[string]string, len(res.GetFileUrls()))
	for _, item := range res.GetFileUrls() {
		if item == nil {
			continue
		}
		urls[item.GetFileId()] = item.GetUrl()
	}

	if len(urls) == 0 {
		return nil
	}

	return urls
}

func (m *fileMapper) ToDeleteFileRequest(fileID string) *filev1.DeleteFileRequest {
	return &filev1.DeleteFileRequest{FileId: fileID}
}

func (m *fileMapper) ToError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return domain.ErrInvalidMediaUpload
	case codes.NotFound:
		return domain.ErrInvalidFileID
	default:
		if errors.Is(err, domain.ErrInvalidMediaUpload) || errors.Is(err, domain.ErrInvalidFileID) {
			return err
		}
		m.log.Error("file-service request failed",
			logging.Operation("grpc.file.map_error"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.Err(err),
		)
		return err
	}
}
