package grpc

import (
	"context"

	"gig-service/config"
	"gig-service/internal/domain"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	filev1 "github.com/ofm-microseervices/ofm-common/proto/file/v1"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	conn *grpcpkg.ClientConn
	cl   filev1.FileServiceClient
	mapr FileMapper
	log  logging.Logger
}

// NewFileService constructs the gRPC client used by gig-service to upload and
// delete file records.
func NewFileService(cfg config.FileServiceConfig, log logging.Logger) (FileService, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyFileServiceAddress
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	conn, err := grpcpkg.NewClient(cfg.Address, grpcpkg.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &client{
		conn: conn,
		cl:   filev1.NewFileServiceClient(conn),
		mapr: newFileMapper(log),
		log:  log.With(logging.String("module", "grpc-file-client"), logging.String("address", cfg.Address)),
	}, nil
}

func (c *client) UploadFiles(ctx context.Context, ownerID, prefix string, files []domain.MediaUpload) ([]string, error) {
	res, err := c.cl.UploadFiles(ctx, c.mapr.ToUploadFilesRequest(ownerID, prefix, files))
	if err != nil {
		return nil, c.mapr.ToError(err)
	}

	return c.mapr.ToUploadFilesResponse(res), nil
}

func (c *client) DeleteFile(ctx context.Context, fileID string) error {
	_, err := c.cl.DeleteFile(ctx, c.mapr.ToDeleteFileRequest(fileID))
	if err != nil {
		return c.mapr.ToError(err)
	}

	return nil
}

// Close closes the underlying gRPC connection.
func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.log.Info("closing file service grpc client")
	return c.conn.Close()
}
