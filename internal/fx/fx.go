package appfx

import "go.uber.org/fx"

// Module is the compatibility bundle of all FX modules used by gig-service.
var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	TracingModule,
	MetricsModule,
	AppModule,
	StorageModule,
	MessagingModule,
	FileClientModule,
	PaymentClientModule,
	UserClientModule,
	AnalyticsModule,
	RepoModule,
	OrderClientModule,
	ReviewClientModule,
	ServiceModule,
	PresentationModule,
)
