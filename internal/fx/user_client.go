package appfx

import (
	"gig-service/config"
	usergrpc "gig-service/internal/infra/user/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"go.uber.org/fx"
)

// UserClientModule wires the user-service gRPC client used for gig bootstrap
// backfills.
var UserClientModule = fx.Options(
	fx.Provide(ProvideUserPreviewClient),
)

// ProvideUserPreviewClient constructs the user-service lookup client.
func ProvideUserPreviewClient(cfg *config.Config, lg logging.Logger) (usergrpc.UserPreviewClient, error) {
	return usergrpc.NewUserPreviewClient(cfg.UserService, lg)
}
