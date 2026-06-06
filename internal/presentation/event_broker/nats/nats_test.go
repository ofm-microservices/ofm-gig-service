package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"gig-service/config"
	app "gig-service/internal/application"
	"gig-service/internal/domain"
	eventbroker "gig-service/internal/presentation/event_broker"
	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type fakeNatsConn struct {
	mu        sync.Mutex
	publishTo []string
	payloads  [][]byte
	handler   nats.MsgHandler
	flushErr  error
	pubErr    error
	subErr    error
}

func (f *fakeNatsConn) Publish(subj string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishTo = append(f.publishTo, subj)
	f.payloads = append(f.payloads, append([]byte(nil), data...))
	return f.pubErr
}

func (f *fakeNatsConn) PublishMsg(msg *nats.Msg) error {
	return f.Publish(msg.Subject, msg.Data)
}

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
func (fakeGigService) GetPublicByID(context.Context, string) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) GetPreviewGigsByFreelancerUsername(context.Context, domain.ListPreviewGigsQuery) (*domain.ListPreviewGigsResult, error) {
	return &domain.ListPreviewGigsResult{}, nil
}
func (fakeGigService) GetMyGigs(context.Context, domain.ListMyGigsQuery) (*domain.ListMyGigsResult, error) {
	return &domain.ListMyGigsResult{}, nil
}
func (fakeGigService) AppendPreviewGig(context.Context, *domain.Gig) error {
	return nil
}
func (fakeGigService) UpsertPreviewGig(context.Context, *domain.Gig) error {
	return nil
}
func (fakeGigService) RefreshPreviewRating(context.Context, string) error {
	return nil
}
func (fakeGigService) RefreshPreviewOrderCount(context.Context, string) error {
	return nil
}
func (fakeGigService) RebuildPopularitySnapshots(context.Context, []*domain.Gig) error {
	return nil
}
func (fakeGigService) GetOrderStartSnapshot(context.Context, string, string) (*app.OrderStartSnapshot, error) {
	return &app.OrderStartSnapshot{GigID: "gig-1", PackageID: "pkg-1", SellerID: "seller-1", GigTitle: "Gig", PackageTitle: "Basic", PackageDescription: "desc", PriceCents: 1, Currency: "usd", DeliveryDays: 1, PackageAvailable: true}, nil
}
func (fakeGigService) Publish(context.Context, string, string, string) (*domain.Gig, error) {
	return &domain.Gig{ID: "gig-1"}, nil
}
func (fakeGigService) Project(_ context.Context, gig *domain.Gig) (*domain.Gig, error) {
	return gig, nil
}

func (f *fakeNatsConn) Subscribe(_ string, cb nats.MsgHandler) (*nats.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subErr != nil {
		return nil, f.subErr
	}
	f.handler = cb
	return nil, nil
}

func (f *fakeNatsConn) FlushWithContext(context.Context) error { return f.flushErr }
func (f *fakeNatsConn) Close()                                 {}

type fakeEventBroker struct {
	cfg     config.PullConsumerConfig
	handler eventbroker.MessageHandler
	err     error
}

func (f *fakeEventBroker) RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	f.cfg = cfg
	f.handler = handler
	_ = ctx
	return f.err
}

func (f *fakeEventBroker) Publish(context.Context, string, []byte) error { return nil }
func (f *fakeEventBroker) Subscribe(context.Context, string, eventbroker.MessageHandler) error {
	return nil
}
func (f *fakeEventBroker) Close() {}

type fakeProjectionWriter struct{ upserts int }

func (w *fakeProjectionWriter) Upsert(context.Context, *domain.Gig) error { w.upserts++; return nil }
func (w *fakeProjectionWriter) DeleteByID(context.Context, string) error  { return nil }

type fakePullRuntime struct{ started bool }

func (r *fakePullRuntime) Start(context.Context) { r.started = true }

type fakeRuntimeFactory struct {
	runtime PullConsumerRuntime
	err     error
	cfg     config.PullConsumerConfig
}

func (f *fakeRuntimeFactory) Create(
	_ *nats.Conn,
	_ logging.Logger,
	cfg config.PullConsumerConfig,
	_ eventbroker.MessageHandler,
) (PullConsumerRuntime, error) {
	f.cfg = cfg
	return f.runtime, f.err
}

type fakeGigMapper struct {
	gig *domain.Gig
	err error
}

func (m *fakeGigMapper) FromPublishedPayload([]byte) (*domain.Gig, error) { return m.gig, m.err }
func (m *fakeGigMapper) ToPublishedPayload(*domain.Gig) ([]byte, error)   { return nil, nil }
func (m *fakeGigMapper) ToViewedPayload(*domain.Gig) ([]byte, error)      { return nil, nil }
func (m *fakeGigMapper) FromReadModelPayload([]byte) (*domain.Gig, error) { return m.gig, m.err }
func (m *fakeGigMapper) ToReadModelPayload(*domain.Gig) ([]byte, error)   { return nil, nil }

var _ = Describe("nats broker helpers", func() {
	var logger logging.Logger

	BeforeEach(func() {
		var err error
		logger, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("validates constructor inputs", func() {
		broker, err := NewBroker(config.NATSConfig{}, logger)
		Expect(broker).To(BeNil())
		Expect(err).To(MatchError(ErrEmptyNATSURL))

		broker, err = NewBroker(config.NATSConfig{URL: "nats://127.0.0.1:1"}, nil)
		Expect(broker).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("wraps connection failures", func() {
		broker, err := NewBroker(config.NATSConfig{URL: "nats://127.0.0.1:1"}, logger)
		Expect(broker).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})

	It("publishes and subscribes through the broker connection", func() {
		conn := &fakeNatsConn{}
		broker := &natsBroker{nc: conn, jsConn: nil, log: logger, validator: newPullConsumerConfigValidator(), runtimeFactory: newPullConsumerRuntimeFactory()}

		Expect(broker.Publish(context.Background(), "subject", []byte("payload"))).To(Succeed())
		Expect(conn.publishTo).To(Equal([]string{"subject"}))
		Expect(conn.payloads[0]).To(Equal([]byte("payload")))

		received := make(chan string, 1)
		Expect(broker.Subscribe(context.Background(), "subject", func(_ context.Context, subject string, payload []byte) error {
			received <- subject + ":" + string(payload)
			return nil
		})).To(Succeed())
		Expect(conn.handler).NotTo(BeNil())
		conn.handler(&nats.Msg{Subject: "subject", Data: []byte("payload")})
		Expect(received).To(Receive(Equal("subject:payload")))
	})

	It("wraps publish and subscribe failures", func() {
		conn := &fakeNatsConn{pubErr: errors.New("pub"), subErr: errors.New("sub")}
		broker := &natsBroker{nc: conn, jsConn: nil, log: logger, validator: newPullConsumerConfigValidator(), runtimeFactory: newPullConsumerRuntimeFactory()}

		err := broker.Publish(context.Background(), "subject", []byte("payload"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("publish to nats"))

		err = broker.Subscribe(context.Background(), "subject", func(context.Context, string, []byte) error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("subscribe to nats"))
	})

	It("wraps flush failures", func() {
		conn := &fakeNatsConn{flushErr: errors.New("flush")}
		broker := &natsBroker{nc: conn, jsConn: nil, log: logger, validator: newPullConsumerConfigValidator(), runtimeFactory: newPullConsumerRuntimeFactory()}

		Expect(broker.Publish(context.Background(), "subject", []byte("payload"))).To(MatchError(ContainSubstring("flush nats publisher")))
		Expect(broker.Subscribe(context.Background(), "subject", func(context.Context, string, []byte) error { return nil })).To(MatchError(ContainSubstring("flush nats publisher")))
	})

	It("flushes using a fallback deadline when none is set", func() {
		conn := &fakeNatsConn{}
		Expect(Flush(context.Background(), conn)).To(Succeed())
	})

	It("resolves adaptive plans", func() {
		cfg := config.PullConsumerConfig{
			BatchSize: 2,
			MaxWait:   20 * time.Millisecond,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				MediumPending:   10,
				HighPending:     20,
				LowBatchSize:    1,
				LowMaxWait:      10 * time.Millisecond,
				MediumBatchSize: 2,
				MediumMaxWait:   5 * time.Millisecond,
				HighBatchSize:   4,
				HighMaxWait:     2 * time.Millisecond,
			},
		}

		tier, batch, waitFor := ResolvePullPlan(cfg, 25)
		Expect(tier).To(Equal("high"))
		Expect(batch).To(Equal(4))
		Expect(waitFor).To(Equal(2 * time.Millisecond))

		tier, batch, waitFor = ResolvePullPlan(cfg, 15)
		Expect(tier).To(Equal("medium"))
		Expect(batch).To(Equal(2))
		Expect(waitFor).To(Equal(5 * time.Millisecond))

		tier, batch, waitFor = ResolvePullPlan(cfg, 5)
		Expect(tier).To(Equal("low"))
		Expect(batch).To(Equal(1))
		Expect(waitFor).To(Equal(10 * time.Millisecond))

		tier, batch, waitFor = ResolvePullPlan(config.PullConsumerConfig{BatchSize: 3, MaxWait: 7 * time.Millisecond}, 1)
		Expect(tier).To(Equal("base"))
		Expect(batch).To(Equal(3))
		Expect(waitFor).To(Equal(7 * time.Millisecond))
	})

	It("wraps helper errors", func() {
		Expect(WrapInitJetStreamContextError(errors.New("boom"))).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureConsumerError("stream", "durable", errors.New("add"), errors.New("update"))).To(MatchError(ContainSubstring("ensure consumer")))
		Expect(WrapCreatePullSubscriberError("subject", "durable", errors.New("boom"))).To(MatchError(ContainSubstring("create pull subscriber")))
		Expect(WrapUnmarshalGigPublishedEventError(errors.New("boom"))).To(MatchError(ContainSubstring("unmarshal gig published event")))
		Expect(WrapMarshalGigPublishedEventError(errors.New("boom"))).To(MatchError(ContainSubstring("marshal gig published event")))
	})

	It("resolves domain failure reasons", func() {
		resolver := NewDomainFailureReasonResolver()
		Expect(resolver.CreateGigFailureReason(domain.ErrInvalidGigID)).To(Equal("invalid gig id"))
		Expect(resolver.CreateGigFailureReason(domain.ErrInvalidFreelancerID)).To(Equal("invalid freelancer id"))
		Expect(resolver.CreateGigFailureReason(domain.ErrInvalidPackageCount)).To(Equal("invalid package count"))
		Expect(resolver.CreateGigFailureReason(domain.ErrInvalidPackageTier)).To(Equal("invalid package tier"))
		Expect(resolver.CreateGigFailureReason(errors.New("boom"))).To(Equal("failed to create gig"))
		Expect(resolver.PublishGigFailureReason(domain.ErrGigDraftIncomplete)).To(Equal("gig draft is incomplete"))
		Expect(resolver.PublishGigFailureReason(domain.ErrGigAlreadyPublished)).To(Equal("gig already published"))
		Expect(resolver.PublishGigFailureReason(errors.New("boom"))).To(Equal("failed to publish gig"))
	})

	It("validates pull consumer configuration", func() {
		validator := newPullConsumerConfigValidator()
		Expect(validator.Validate(config.PullConsumerConfig{})).To(MatchError(ErrEmptyStreamName))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s"})).To(MatchError(ErrEmptySubject))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub"})).To(MatchError(ErrEmptyDurableName))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d"})).To(MatchError(ErrInvalidBatchSize))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1})).To(MatchError(ErrInvalidMaxWait))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond})).To(MatchError(ErrInvalidWorkerCount))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1})).To(MatchError(ErrInvalidQueueSize))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1})).To(MatchError(ErrInvalidAckWait))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Millisecond})).To(MatchError(ErrInvalidMaxDeliver))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Millisecond, MaxDeliver: 1})).To(Succeed())
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Millisecond, MaxDeliver: 1, Adaptive: config.PullAdaptiveConfig{Enabled: true, CheckInterval: time.Millisecond, MediumPending: 1, HighPending: 1}})).To(MatchError(ErrInvalidAdaptiveThresholds))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "s", Subject: "sub", Durable: "d", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Millisecond, MaxDeliver: 1, Adaptive: config.PullAdaptiveConfig{Enabled: true, CheckInterval: time.Millisecond, MediumPending: 1, HighPending: 2}})).To(MatchError(ErrInvalidAdaptivePlan))
	})

	It("subscribes the projection subscriber through the broker", func() {
		broker := &fakeEventBroker{}
		ctrl := gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		writer := NewMockProjectionWriter(ctrl)
		mapper := app.NewGigEventMapper()
		subscriber, err := NewGigProjectionSubscriber(broker, fakeGigService{}, writer, mapper, config.NATSConfig{
			GigEventsStream:              "GIG_EVENTS",
			GigPublishedSubject:          "gig.published",
			GigProjectionSubject:         "gig.projection.requested",
			GigProjectionBatchSize:       7,
			GigProjectionMaxWait:         8 * time.Second,
			GigProjectionWorkers:         3,
			GigProjectionDurable:         "durable",
			GigProjectionQueueSize:       11,
			GigProjectionAckWait:         12 * time.Second,
			GigProjectionMaxDeliver:      5,
			GigProjectionAdaptiveEnabled: true,
		}, logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(subscriber.Subscribe(context.Background())).To(Succeed())
		Expect(broker.cfg.Stream).To(Equal("GIG_EVENTS"))
		Expect(broker.cfg.Subject).To(Equal("gig.projection.requested"))
		Expect(broker.cfg.BatchSize).To(Equal(7))
	})

	It("rejects nil projection dependencies", func() {
		_, err := NewGigProjectionSubscriber(nil, fakeGigService{}, &fakeProjectionWriter{}, app.NewGigEventMapper(), config.NATSConfig{}, logger)
		Expect(err).To(MatchError(ErrNilBroker))

		_, err = NewGigProjectionSubscriber(&fakeEventBroker{}, fakeGigService{}, nil, app.NewGigEventMapper(), config.NATSConfig{}, logger)
		Expect(err).To(MatchError(ErrNilProjectionWriter))

		_, err = NewGigProjectionSubscriber(&fakeEventBroker{}, fakeGigService{}, &fakeProjectionWriter{}, app.NewGigEventMapper(), config.NATSConfig{}, nil)
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("handles published payloads", func() {
		ctrl := gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		writer := NewMockProjectionWriter(ctrl)
		subscriber, err := NewGigProjectionSubscriber(&fakeEventBroker{}, fakeGigService{}, writer, app.NewGigEventMapper(), config.NATSConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())
		impl := subscriber.(*gigProjectionSubscriber)

		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte("bad"))).To(HaveOccurred())
		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte(`{"gig_id":""}`))).To(MatchError(ErrInvalidGigPayload))

		payload, err := app.NewGigEventMapper().ToPublishedPayload(&domain.Gig{ID: "gig-1", FreelancerID: "freelancer-1", Status: domain.StatusDraft, BasicInfoCompleted: true, PackagesCompleted: true, RequirementsCompleted: true, MediaCompleted: true, Packages: []domain.GigPackage{{Tier: domain.TierBasic, Description: "basic", DeliveryDays: 1, PriceCents: 1}}})
		Expect(err).NotTo(HaveOccurred())
		writer.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", payload)).To(Succeed())
	})

	It("handles invalid payloads and writer failures", func() {
		ctrl := gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		writer := NewMockProjectionWriter(ctrl)
		subscriber, err := NewGigProjectionSubscriber(&fakeEventBroker{}, fakeGigService{}, writer, &fakeGigMapper{}, config.NATSConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())
		impl := subscriber.(*gigProjectionSubscriber)

		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte("payload"))).To(MatchError("invalid gig payload"))

		impl.mapr = &fakeGigMapper{gig: nil}
		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte("payload"))).To(MatchError("invalid gig payload"))

		impl.mapr = &fakeGigMapper{gig: &domain.Gig{}}
		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte("payload"))).To(MatchError("invalid gig payload"))

		impl.mapr = &fakeGigMapper{gig: &domain.Gig{ID: "gig-1"}}
		writer.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(errors.New("write"))
		Expect(impl.handleGigProjectionRequestedEvent(context.Background(), "gig.projection.requested", []byte("payload"))).To(MatchError(ContainSubstring("write")))
	})

	It("runs a pull consumer when validation and runtime creation succeed", func() {
		runtime := &fakePullRuntime{}
		factory := &fakeRuntimeFactory{runtime: runtime}
		broker := &natsBroker{log: logger, validator: newPullConsumerConfigValidator(), runtimeFactory: factory}
		cfg := config.PullConsumerConfig{
			Stream:     "GIG_EVENTS",
			Subject:    "gig.published",
			Durable:    "durable",
			BatchSize:  1,
			MaxWait:    time.Millisecond,
			Workers:    1,
			QueueSize:  1,
			AckWait:    time.Millisecond,
			MaxDeliver: 1,
		}

		Expect(broker.RunPullConsumer(context.Background(), cfg, func(context.Context, string, []byte) error { return nil })).To(Succeed())
		Expect(factory.cfg).To(Equal(cfg))
		Expect(runtime.started).To(BeTrue())
	})

	It("returns runtime factory errors", func() {
		broker := &natsBroker{log: logger, validator: newPullConsumerConfigValidator(), runtimeFactory: &fakeRuntimeFactory{err: errors.New("factory")}}
		cfg := config.PullConsumerConfig{
			Stream:     "GIG_EVENTS",
			Subject:    "gig.published",
			Durable:    "durable",
			BatchSize:  1,
			MaxWait:    time.Millisecond,
			Workers:    1,
			QueueSize:  1,
			AckWait:    time.Millisecond,
			MaxDeliver: 1,
		}

		Expect(broker.RunPullConsumer(context.Background(), cfg, func(context.Context, string, []byte) error { return nil })).To(MatchError("factory"))
	})
})
