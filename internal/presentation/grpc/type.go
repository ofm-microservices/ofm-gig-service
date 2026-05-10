package grpc

import (
	"context"

	app "gig-service/internal/application"
)

// GigService exposes the application-level gig draft workflow to the gRPC
// transport.
type GigService = app.GigService

// Server defines the gRPC server lifecycle exposed by gig-service.
type Server interface {
	Start() error
	Shutdown(ctx context.Context) error
}
