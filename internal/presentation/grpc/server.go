package grpc

import (
	"context"
	"fmt"
	"gig-service/config"
	"gig-service/internal/domain"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	gigv1 "github.com/ofm-microservices/ofm-common/proto/gig/v1"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"time"
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

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()),
	)
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
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.CreateDraft(ctx, req.GetFreelancerId())
	if err != nil {
		log.Error("create draft failed",
			logging.Operation("grpc.gig.create_draft"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return s.mapr.ToCreateDraftResponse(gig), nil
}

// UpdateBasicInfo updates the gig's public metadata.
func (s *server) UpdateBasicInfo(ctx context.Context, req *gigv1.UpdateBasicInfoRequest) (*gigv1.UpdateBasicInfoResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.UpdateBasicInfo(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToUpdateBasicInfoParams(req))
	if err != nil {
		log.Error("update basic info failed",
			logging.Operation("grpc.gig.update_basic_info"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.UpdateBasicInfoResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplacePackages replaces all gig package tiers.
func (s *server) ReplacePackages(ctx context.Context, req *gigv1.ReplacePackagesRequest) (*gigv1.ReplacePackagesResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.ReplacePackages(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplacePackagesParams(req))
	if err != nil {
		log.Error("replace packages failed",
			logging.Operation("grpc.gig.replace_packages"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplacePackagesResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplaceQuestions replaces the gig requirements questions.
func (s *server) ReplaceQuestions(ctx context.Context, req *gigv1.ReplaceQuestionsRequest) (*gigv1.ReplaceQuestionsResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.ReplaceQuestions(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplaceQuestionsParams(req))
	if err != nil {
		log.Error("replace questions failed",
			logging.Operation("grpc.gig.replace_questions"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplaceQuestionsResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// ReplaceMedia replaces the gig media references.
func (s *server) ReplaceMedia(ctx context.Context, req *gigv1.ReplaceMediaRequest) (*gigv1.ReplaceMediaResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.ReplaceMedia(ctx, req.GetGigId(), req.GetFreelancerId(), s.mapr.ToReplaceMediaParams(req))
	if err != nil {
		log.Error("replace media failed",
			logging.Operation("grpc.gig.replace_media"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.ReplaceMediaResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// GetDraft returns the current gig draft state.
func (s *server) GetDraft(ctx context.Context, req *gigv1.GetDraftRequest) (*gigv1.GetDraftResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.GetByID(ctx, req.GetGigId(), req.GetFreelancerId())
	if err != nil {
		log.Error("get draft failed",
			logging.Operation("grpc.gig.get_draft"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.GetDraftResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

// GetOrderStartSnapshot returns the published gig snapshot for the order wizard.
func (s *server) GetOrderStartSnapshot(ctx context.Context, req *gigv1.GetOrderStartSnapshotRequest) (*gigv1.GetOrderStartSnapshotResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	snapshot, err := s.svc.GetOrderStartSnapshot(ctx, req.GetGigId(), req.GetPackageId())
	if err != nil {
		log.Error("get order start snapshot failed",
			logging.Operation("grpc.gig.get_order_start_snapshot"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("package_id", req.GetPackageId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.GetOrderStartSnapshotResponse{Snapshot: s.mapr.ToOrderStartSnapshot(snapshot)}, nil
}

// GetGigBySlug returns the public gig detail view for the provided slug.
func (s *server) GetGigBySlug(ctx context.Context, req *gigv1.GetGigBySlugRequest) (*gigv1.GetGigBySlugResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gigID, err := parseGigIDFromSlug(req.GetSlug())
	if err != nil {
		log.Error("get gig by slug failed",
			logging.Operation("grpc.gig.get_by_slug"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("slug", req.GetSlug()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	gig, err := s.svc.GetPublicByID(ctx, gigID)
	if err != nil {
		log.Error("get gig by slug failed",
			logging.Operation("grpc.gig.get_by_slug"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("slug", req.GetSlug()),
			logging.String("gig_id", gigID),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return s.mapr.ToGetGigBySlugResponse(gig), nil
}

// Publish makes the gig visible and emits the published event.
func (s *server) Publish(ctx context.Context, req *gigv1.PublishRequest) (*gigv1.PublishResponse, error) {
	started := time.Now()
	log := logging.WithContext(ctx, s.log)
	gig, err := s.svc.Publish(ctx, req.GetGigId(), req.GetFreelancerId())
	if err != nil {
		log.Error("publish failed",
			logging.Operation("grpc.gig.publish"),
			logging.Attempt(1),
			logging.Retryable(false),
			logging.DurationMS(time.Since(started)),
			logging.String("gig_id", req.GetGigId()),
			logging.String("freelancer_id", req.GetFreelancerId()),
			logging.Err(err),
		)
		return nil, s.mapr.ToError(err)
	}

	return &gigv1.PublishResponse{Gig: s.mapr.ToGigResponse(gig)}, nil
}

func parseGigIDFromSlug(slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", domain.ErrInvalidGigSlug
	}
	if len(slug) <= 37 {
		return "", domain.ErrInvalidGigSlug
	}
	if slug[len(slug)-37] != '-' {
		return "", domain.ErrInvalidGigSlug
	}
	gigID := slug[len(slug)-36:]
	parsed, err := uuid.Parse(gigID)
	if err != nil || parsed.Version() != 7 {
		return "", domain.ErrInvalidGigSlug
	}
	return gigID, nil
}
