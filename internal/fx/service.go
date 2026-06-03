package appfx

import (
	"gig-service/config"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	filegrpc "gig-service/internal/infra/file/grpc"
	paymentgrpc "gig-service/internal/infra/payment/grpc"
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
func ProvideGigService(cfg *config.Config, writeRepo domain.GigRepository, readRepo domain.GigReadRepository, files filegrpc.FileService, connect paymentgrpc.PaymentService, broker eventbroker.EventBroker, pop app.PopularitySource, slugger app.Slugger, lg logging.Logger) (app.GigService, error) {
	return app.New(writeRepo, readRepo, files, connect, broker, pop, slugger, lg, cfg.Preview)
}
