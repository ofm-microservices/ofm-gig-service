package grpc

import (
	"context"
	"fmt"
	"gig-service/config"
	"net"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	gigv1 "github.com/ofm-microservices/ofm-common/proto/gig/v1"
	"google.golang.org/grpc"
)

type server struct {
	gigv1.UnimplementedGigCommandServiceServer
	svc      GigService
	cfg      config.GRPCConfig
	log      logging.Logger
	mapr     GigMapper
	srv      *grpc.Server
	listener net.Listener
}

// NewServer constructs the gig-service gRPC draft workflow server.
func NewServer(svc GigService, cfg config.GRPCConfig, log logging.Logger) (Server, error) {
	if svc == nil {
		return nil, ErrNilGigService
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()))
	s := &server{
		svc:  svc,
		cfg:  cfg,
		mapr: newGigMapper(log.With(logging.String("module", "grpc-mapper"))),
		log:  log.With(logging.String("module", "grpc-server")),
		srv:  grpcSrv,
	}
	gigv1.RegisterGigCommandServiceServer(grpcSrv, s)
	return s, nil
}

// Start begins serving gRPC traffic on the configured address.
func (s *server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.listener = lis
	s.log.Info("starting grpc server", logging.String("addr", addr))
	return s.srv.Serve(lis)
}

// Shutdown gracefully stops the gRPC server.
func (s *server) Shutdown(context.Context) error {
	if s.srv != nil {
		s.log.Info("shutting down grpc server")
		s.srv.GracefulStop()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// CreateDraft creates a server-side gig draft for the authenticated
// freelancer.
func (s *server) CreateDraft(ctx context.Context, req *gigv1.CreateDraftRequest) (*gigv1.CreateDraftResponse, error) {
	gig, err := s.svc.CreateDraft(ctx, req.GetFreelancerId())
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return s.mapr.ToCreateDraftResponse(gig), nil
}

// UpdateBasicInfo updates the gig's public metadata.
func (s *server) UpdateBasicInfo(ctx context.Context, req *gigv1.UpdateBasicInfoRequest) (*gigv1.UpdateBasicInfoResponse, error) {
	gig, err := s.svc.UpdateBasicInfo(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToUpdateBasicInfoParams(req))
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.UpdateBasicInfoResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplacePackages replaces all gig package tiers.
func (s *server) ReplacePackages(ctx context.Context, req *gigv1.ReplacePackagesRequest) (*gigv1.ReplacePackagesResponse, error) {
	gig, err := s.svc.ReplacePackages(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplacePackagesParams(req))
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplacePackagesResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplaceQuestions replaces the gig requirements questions.
func (s *server) ReplaceQuestions(ctx context.Context, req *gigv1.ReplaceQuestionsRequest) (*gigv1.ReplaceQuestionsResponse, error) {
	gig, err := s.svc.ReplaceQuestions(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplaceQuestionsParams(req))
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplaceQuestionsResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplaceMedia replaces the gig media references.
func (s *server) ReplaceMedia(ctx context.Context, req *gigv1.ReplaceMediaRequest) (*gigv1.ReplaceMediaResponse, error) {
	gig, err := s.svc.ReplaceMedia(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplaceMediaParams(req))
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplaceMediaResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// GetDraft returns the current gig draft state.
func (s *server) GetDraft(ctx context.Context, req *gigv1.GetDraftRequest) (*gigv1.GetDraftResponse, error) {
	gig, err := s.svc.GetByID(ctx, req.GetGigId(), req.GetFreelancerId())
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.GetDraftResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// Publish makes the gig visible and emits the published event.
func (s *server) Publish(ctx context.Context, req *gigv1.PublishRequest) (*gigv1.PublishResponse, error) {
	gig, err := s.svc.Publish(ctx, req.GetGigId(), req.GetFreelancerId())
	if err != nil {
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.PublishResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}
