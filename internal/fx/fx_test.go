package appfx

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"testing"

	"gig-service/config"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	filegrpc "gig-service/internal/infra/file/grpc"
	eventbroker "gig-service/internal/presentation/event_broker"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

type fakeLifecycle struct {
	hooks []fx.Hook
}

func (l *fakeLifecycle) Append(h fx.Hook) {
	l.hooks = append(l.hooks, h)
}

type fakeLogger struct {
	mu     sync.Mutex
	infos  []string
	errors []string
}

func (l *fakeLogger) Debug(string, ...logging.Field) {}
func (l *fakeLogger) Warn(string, ...logging.Field)  {}
func (l *fakeLogger) Error(msg string, _ ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}
func (l *fakeLogger) Info(msg string, _ ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}
func (l *fakeLogger) With(_ ...logging.Field) logging.Logger { return l }
func (l *fakeLogger) Sync() error                            { return nil }

type fakeEventBroker struct {
	closed bool
}

func (f *fakeEventBroker) Publish(context.Context, string, []byte) error { return nil }
func (f *fakeEventBroker) Subscribe(context.Context, string, eventbroker.MessageHandler) error {
	return nil
}
func (f *fakeEventBroker) RunPullConsumer(context.Context, config.PullConsumerConfig, eventbroker.MessageHandler) error {
	return nil
}
func (f *fakeEventBroker) Close() { f.closed = true }

type fakeProjectionWriter struct{ upserts int }

func (w *fakeProjectionWriter) Upsert(context.Context, *domain.Gig) error { w.upserts++; return nil }
func (w *fakeProjectionWriter) DeleteByID(context.Context, string) error  { return nil }

type fakeGigService struct{}

func (fakeGigService) CreateDraft(context.Context, string) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) UpdateBasicInfo(context.Context, string, string, domain.UpdateBasicInfoParams) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) ReplacePackages(context.Context, string, string, domain.ReplacePackagesParams) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) ReplaceQuestions(context.Context, string, string, domain.ReplaceQuestionsParams) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) ReplaceMedia(context.Context, string, string, domain.ReplaceMediaUploadParams) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) GetByID(context.Context, string, string) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) Publish(context.Context, string, string) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}

type fakeServer struct {
	started  bool
	stopped  bool
	startErr error
}

func (s *fakeServer) Start() error {
	s.started = true
	return s.startErr
}
func (s *fakeServer) Shutdown(context.Context) error {
	s.stopped = true
	return nil
}

func TestFX(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gig FX Suite")
}

var _ = Describe("fx providers and invokes", func() {
	var lg *fakeLogger

	BeforeEach(func() {
		lg = &fakeLogger{}
	})

	It("starts with the basic logger hook", func() {
		InvokeStartLog(lg)
		Expect(lg.infos).To(ContainElement("starting gig-service"))
	})

	It("loads config from the environment", func() {
		GinkgoT().Setenv("APP_ENV", "test")
		GinkgoT().Setenv("LOG_LEVEL", "debug")
		GinkgoT().Setenv("DB_HOST", "db")
		GinkgoT().Setenv("DB_PORT", "5433")
		GinkgoT().Setenv("DB_USER", "user")
		GinkgoT().Setenv("DB_PASSWORD", "pass")
		GinkgoT().Setenv("DB_NAME", "gig_service")
		GinkgoT().Setenv("GRPC_PORT", "9503")
		GinkgoT().Setenv("REDIS_HOST", "redis")
		GinkgoT().Setenv("REDIS_PORT", "6380")
		GinkgoT().Setenv("NATS_URL", "nats://127.0.0.1:4222")
		GinkgoT().Setenv("FILE_SERVICE_ADDRESS", "127.0.0.1:9096")
		GinkgoT().Setenv("PAYMENT_SERVICE_ADDRESS", "127.0.0.1:9097")

		cfg, err := ProvideConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.App.Env).To(Equal("test"))
	})

	It("provides a logger and syncs on stop", func() {
		lc := &fakeLifecycle{}
		cfg := &config.Config{App: config.AppConfig{Env: "test", LogLevel: "debug"}}
		lgOut, err := ProvideLogger(lc, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(lgOut).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop(context.Background())).To(HaveOccurred())
	})

	It("returns logger construction errors", func() {
		lgOut, err := ProvideLogger(&fakeLifecycle{}, &config.Config{App: config.AppConfig{Env: "test", LogLevel: "not-a-level"}})
		Expect(lgOut).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("constructs and closes the file-service client", func() {
		lc := &fakeLifecycle{}
		client, err := ProvideFileServiceClient(lc, &config.Config{FileService: config.FileServiceConfig{Address: "127.0.0.1:1"}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())
		var _ filegrpc.FileService = client
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
	})

	It("returns file-service client construction errors", func() {
		client, err := ProvideFileServiceClient(&fakeLifecycle{}, &config.Config{FileService: config.FileServiceConfig{}}, lg)
		Expect(client).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("boots the migration and storage providers", func() {
		originalRun := runMigrations
		originalOpenYB := openYugaByteDB
		originalOpenRedis := openRedisClient
		DeferCleanup(func() {
			runMigrations = originalRun
			openYugaByteDB = originalOpenYB
			openRedisClient = originalOpenRedis
		})

		called := false
		runMigrations = func(config.DBConfig) error { called = true; return nil }
		Expect(InvokeRunMigrations(&config.Config{DB: config.DBConfig{}}, lg)).To(Succeed())
		Expect(called).To(BeTrue())

		raw, mock, err := sqlxMock()
		Expect(err).NotTo(HaveOccurred())
		defer raw.Close()
		_ = mock
		openYugaByteDB = func(config.DBConfig) (*sqlx.DB, error) { return sqlx.NewDb(raw, "sqlmock"), nil }
		lc := &fakeLifecycle{}
		dbx, err := ProvideYugaByteDB(lc, &config.Config{DB: config.DBConfig{}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbx).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())

		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()
		port, err := strconv.Atoi(srv.Port())
		Expect(err).NotTo(HaveOccurred())
		openRedisClient = func(context.Context, config.RedisConfig) (*redis.Client, error) {
			return redis.NewClient(&redis.Options{Addr: srv.Addr()}), nil
		}
		lc = &fakeLifecycle{}
		rdb, err := ProvideRedisClient(lc, &config.Config{Redis: config.RedisConfig{Host: "127.0.0.1", Port: port}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(rdb).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
	})

	It("reports storage provider failures", func() {
		originalRun := runMigrations
		originalOpenYB := openYugaByteDB
		originalOpenRedis := openRedisClient
		DeferCleanup(func() {
			runMigrations = originalRun
			openYugaByteDB = originalOpenYB
			openRedisClient = originalOpenRedis
		})

		runMigrations = func(config.DBConfig) error { return errors.New("boom") }
		Expect(InvokeRunMigrations(&config.Config{DB: config.DBConfig{}}, lg)).To(MatchError(ContainSubstring("boom")))

		openYugaByteDB = func(config.DBConfig) (*sqlx.DB, error) { return nil, errors.New("db") }
		dbx, err := ProvideYugaByteDB(&fakeLifecycle{}, &config.Config{DB: config.DBConfig{}}, lg)
		Expect(dbx).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("db")))

		openRedisClient = func(context.Context, config.RedisConfig) (*redis.Client, error) { return nil, errors.New("redis") }
		rdb, err := ProvideRedisClient(&fakeLifecycle{}, &config.Config{Redis: config.RedisConfig{}}, lg)
		Expect(rdb).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("redis")))
	})

	It("provides event broker, subscribers, and services", func() {
		originalEnsure := ensureStream
		originalBroker := newEventBroker
		DeferCleanup(func() {
			ensureStream = originalEnsure
			newEventBroker = originalBroker
		})

		ensureCalled := false
		ensureStream = func(config.NATSConfig, logging.Logger) error {
			ensureCalled = true
			return nil
		}
		Expect(InvokeEnsureStream(&config.Config{NATS: config.NATSConfig{URL: "nats://127.0.0.1:4222"}}, lg)).To(Succeed())
		Expect(ensureCalled).To(BeTrue())

		broker := &fakeEventBroker{}
		newEventBroker = func(config.NATSConfig, logging.Logger) (eventbroker.EventBroker, error) { return broker, nil }
		lc := &fakeLifecycle{}
		brokerOut, err := ProvideEventBroker(lc, &config.Config{NATS: config.NATSConfig{URL: "nats://127.0.0.1:4222"}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(brokerOut).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
		Expect(broker.closed).To(BeTrue())

		subscriber, err := ProvideGigProjectionSubscriber(broker, &fakeProjectionWriter{}, app.NewGigEventMapper(), &config.Config{NATS: config.NATSConfig{GigEventsStream: "GIG_EVENTS", GigPublishedSubject: "gig.published"}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(subscriber).NotTo(BeNil())

		srv, err := ProvideGRPCServer(fakeGigService{}, &config.Config{GRPC: config.GRPCConfig{}}, lg)
		Expect(err).NotTo(HaveOccurred())
		Expect(srv).NotTo(BeNil())
	})

	It("reports messaging and presentation provider failures", func() {
		originalEnsure := ensureStream
		originalBroker := newEventBroker
		DeferCleanup(func() {
			ensureStream = originalEnsure
			newEventBroker = originalBroker
		})

		ensureStream = func(config.NATSConfig, logging.Logger) error { return errors.New("ensure") }
		Expect(InvokeEnsureStream(&config.Config{NATS: config.NATSConfig{URL: "nats://127.0.0.1:4222"}}, lg)).To(MatchError(ContainSubstring("ensure")))

		newEventBroker = func(config.NATSConfig, logging.Logger) (eventbroker.EventBroker, error) {
			return nil, errors.New("broker")
		}
		brokerOut, err := ProvideEventBroker(&fakeLifecycle{}, &config.Config{NATS: config.NATSConfig{URL: "nats://127.0.0.1:4222"}}, lg)
		Expect(brokerOut).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("broker")))

		subscriber := &fakeSubscriber{err: errors.New("subscribe")}
		lc := &fakeLifecycle{}
		InvokeSubscribeGigProjection(lc, subscriber, &config.Config{App: config.AppConfig{Env: "test"}}, lg)
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStart(context.Background())).To(MatchError(ContainSubstring("subscribe")))

		srv, err := ProvideGRPCServer(nil, &config.Config{GRPC: config.GRPCConfig{}}, lg)
		Expect(srv).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("wires the service and repository providers", func() {
		repo, err := ProvideWriteRepo(nil, nil, lg)
		Expect(repo).To(BeNil())
		Expect(err).To(HaveOccurred())

		readRepo, err := ProvideReadRepo(nil, nil, lg)
		Expect(readRepo).To(BeNil())
		Expect(err).To(HaveOccurred())

		svc, err := ProvideGigService(nil, nil, nil, nil, nil, lg)
		Expect(svc).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("starts and stops the projection and grpc hooks", func() {
		subscriber := &fakeSubscriber{}
		lc := &fakeLifecycle{}
		InvokeSubscribeGigProjection(lc, subscriber, &config.Config{App: config.AppConfig{Env: "test"}}, lg)
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStart(context.Background())).To(Succeed())
		Expect(subscriber.started).To(BeTrue())
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())

		server := &fakeServer{}
		lc = &fakeLifecycle{}
		InvokeRunGRPCServer(lc, server)
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStart(context.Background())).To(Succeed())
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
	})
})

type fakeSubscriber struct {
	started bool
	err     error
}

func (s *fakeSubscriber) Subscribe(context.Context) error {
	s.started = true
	return s.err
}

func sqlxMock() (*sql.DB, sqlmock.Sqlmock, error) { return sqlmock.New() }
