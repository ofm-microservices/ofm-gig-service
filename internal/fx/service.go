package appfx

import (
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	filegrpc "gig-service/internal/infra/file/grpc"
	eventbroker "gig-service/internal/presentation/event_broker"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// ServiceModule provides the application service used by gig-service.
var ServiceModule = fx.Options(
	fx.Provide(
		app.NewSlugger,
		app.NewGigEventMapper,
		ProvideGigService,
	),
)

// ProvideGigService constructs the gig-service application service.
func ProvideGigService(writeRepo domain.GigRepository, files filegrpc.FileService, broker eventbroker.EventBroker, slugger app.Slugger, lg logging.Logger) (app.GigService, error) {
	return app.New(writeRepo, files, broker, slugger, lg)
}
