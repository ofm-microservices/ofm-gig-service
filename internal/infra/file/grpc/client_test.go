package grpc

import (
	"context"
	"errors"

	"gig-service/config"
	"gig-service/internal/domain"
	filev1 "github.com/ofm-microseervices/ofm-common/proto/file/v1"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeFileServiceClient struct {
	uploadReq *filev1.UploadFilesRequest
	deleteReq *filev1.DeleteFileRequest
	uploadRes *filev1.UploadFilesResponse
	uploadErr error
	deleteErr error
}

func (f *fakeFileServiceClient) UploadFile(context.Context, *filev1.UploadFileRequest, ...grpc.CallOption) (*filev1.UploadFileResponse, error) {
	return nil, nil
}

func (f *fakeFileServiceClient) UploadFiles(_ context.Context, req *filev1.UploadFilesRequest, _ ...grpc.CallOption) (*filev1.UploadFilesResponse, error) {
	f.uploadReq = req
	return f.uploadRes, f.uploadErr
}

func (f *fakeFileServiceClient) GetFile(context.Context, *filev1.GetFileRequest, ...grpc.CallOption) (*filev1.GetFileResponse, error) {
	return nil, nil
}

func (f *fakeFileServiceClient) DeleteFile(_ context.Context, req *filev1.DeleteFileRequest, _ ...grpc.CallOption) (*filev1.DeleteFileResponse, error) {
	f.deleteReq = req
	return nil, f.deleteErr
}

var _ = Describe("file gRPC mapper", func() {
	var lg logging.Logger

	BeforeEach(func() {
		var err error
		lg, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("maps requests, responses, and status codes", func() {
		mapper := newFileMapper(lg)
		req := mapper.ToUploadFilesRequest("owner-1", "prefix", []domain.MediaUpload{{Filename: "file", ContentType: "image/jpeg", Data: []byte("x")}})
		Expect(req.OwnerId).To(Equal("owner-1"))
		Expect(req.Prefix).To(Equal("prefix"))
		Expect(req.Files[0].Filename).To(Equal("file"))

		Expect(mapper.ToUploadFilesResponse(&filev1.UploadFilesResponse{Files: []*filev1.File{{FileId: "file-1"}}})).To(Equal([]string{"file-1"}))
		Expect(mapper.ToUploadFilesResponse(nil)).To(BeNil())
		Expect(mapper.ToDeleteFileRequest("file-1").FileId).To(Equal("file-1"))
		Expect(mapper.ToError(status.Error(codes.InvalidArgument, "bad"))).To(MatchError(domain.ErrInvalidMediaUpload))
		Expect(mapper.ToError(status.Error(codes.NotFound, "missing"))).To(MatchError(domain.ErrInvalidFileID))
		Expect(mapper.ToError(domain.ErrInvalidFileID)).To(MatchError(domain.ErrInvalidFileID))
		Expect(mapper.ToError(errors.New("boom"))).To(MatchError("boom"))
	})
})

var _ = Describe("file gRPC client", func() {
	var logger logging.Logger

	BeforeEach(func() {
		var err error
		logger, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("validates constructor inputs", func() {
		client, err := NewFileService(config.FileServiceConfig{}, logger)
		Expect(client).To(BeNil())
		Expect(err).To(MatchError(ErrEmptyFileServiceAddress))

		client, err = NewFileService(config.FileServiceConfig{Address: "127.0.0.1:1"}, nil)
		Expect(client).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("constructs a client and closes it", func() {
		client, err := NewFileService(config.FileServiceConfig{Address: "127.0.0.1:1"}, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Close()).To(Succeed())
	})

	It("uploads and deletes through the mapped gRPC client", func() {
		mapper := newFileMapper(logger)
		fake := &fakeFileServiceClient{
			uploadRes: &filev1.UploadFilesResponse{Files: []*filev1.File{{FileId: "cover"}, {FileId: "gallery"}}},
		}
		client := &client{
			cl:   fake,
			mapr: mapper,
			log:  logger,
		}

		ids, err := client.UploadFiles(context.Background(), "owner-1", "gigs/gig-1/media", []domain.MediaUpload{{Filename: "cover.jpg", ContentType: "image/jpeg", Data: []byte("x")}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"cover", "gallery"}))
		Expect(fake.uploadReq.OwnerId).To(Equal("owner-1"))
		Expect(fake.uploadReq.Prefix).To(Equal("gigs/gig-1/media"))

		Expect(client.DeleteFile(context.Background(), "cover")).To(Succeed())
		Expect(fake.deleteReq.FileId).To(Equal("cover"))
	})

	It("maps upstream errors", func() {
		fake := &fakeFileServiceClient{
			uploadErr: status.Error(codes.InvalidArgument, "bad"),
			deleteErr: status.Error(codes.NotFound, "missing"),
		}
		client := &client{cl: fake, mapr: newFileMapper(logger), log: logger}

		ids, err := client.UploadFiles(context.Background(), "owner-1", "prefix", []domain.MediaUpload{{Filename: "file", ContentType: "image/jpeg", Data: []byte("x")}})
		Expect(ids).To(BeNil())
		Expect(err).To(MatchError(domain.ErrInvalidMediaUpload))

		Expect(client.DeleteFile(context.Background(), "file")).To(MatchError(domain.ErrInvalidFileID))
	})

	It("closes nil clients safely", func() {
		var nilClient *client
		Expect(nilClient.Close()).To(Succeed())
		Expect((&client{}).Close()).To(Succeed())
	})
})
