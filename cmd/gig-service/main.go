package main

import (
	appfx "gig-service/internal/fx"

	"go.uber.org/fx"
)

func newApp() *fx.App {
	return fx.New(
		appfx.ConfigModule,
		appfx.LoggerModule,
		appfx.TracingModule,
		appfx.MetricsModule,
		appfx.AppModule,
		appfx.StorageModule,
		appfx.MessagingModule,
		appfx.FileClientModule,
		appfx.RepoModule,
		appfx.ServiceModule,
		appfx.PresentationModule,
	)
}

var runApp = (*fx.App).Run

func main() {
	runApp(newApp())
}
