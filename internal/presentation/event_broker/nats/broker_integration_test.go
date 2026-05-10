package nats

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"gig-service/config"
	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var brokerIntegrationContainer testcontainers.Container
var brokerIntegrationURL string
var brokerIntegrationLogger logging.Logger

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var err error
	brokerIntegrationLogger, err = logging.New("gig-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())

	brokerIntegrationContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.12.4-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := brokerIntegrationContainer.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := brokerIntegrationContainer.MappedPort(ctx, "4222/tcp")
	Expect(err).NotTo(HaveOccurred())
	brokerIntegrationURL = fmt.Sprintf("nats://%s:%s", host, port.Port())
})

var _ = AfterSuite(func() {
	if brokerIntegrationContainer != nil {
		Expect(brokerIntegrationContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("nats broker integration", func() {
	var cfg config.NATSConfig

	BeforeEach(func() {
		cfg = config.NATSConfig{
			URL:                 brokerIntegrationURL,
			GigEventsStream:     "GIG_EVENTS",
			GigPublishedSubject: "gig.published",
			GigProjectionDurable: "gig_projection",
			GigProjectionBatchSize: 2,
			GigProjectionMaxWait: 50 * time.Millisecond,
			GigProjectionWorkers: 1,
			GigProjectionQueueSize: 4,
			GigProjectionAckWait: 2 * time.Second,
			GigProjectionMaxDeliver: 3,
			GigProjectionAdaptiveEnabled: true,
			GigProjectionAdaptiveCheckInterval: time.Millisecond,
			GigProjectionAdaptiveMediumPending: 1,
			GigProjectionAdaptiveHighPending: 2,
			GigProjectionAdaptiveLowBatchSize: 1,
			GigProjectionAdaptiveLowMaxWait: 20 * time.Millisecond,
			GigProjectionAdaptiveMediumBatchSize: 2,
			GigProjectionAdaptiveMediumMaxWait: 10 * time.Millisecond,
			GigProjectionAdaptiveHighBatchSize: 3,
			GigProjectionAdaptiveHighMaxWait: 5 * time.Millisecond,
		}

		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		js, err := rawConn.JetStream()
		Expect(err).NotTo(HaveOccurred())
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      cfg.GigEventsStream,
			Subjects:  []string{cfg.GigPublishedSubject},
			Storage:   nats.FileStorage,
			Retention: nats.LimitsPolicy,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("publishes and subscribes core messages", func() {
		brokerAny, err := NewBroker(cfg, brokerIntegrationLogger)
		Expect(err).NotTo(HaveOccurred())
		defer brokerAny.Close()

		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		sub, err := rawConn.SubscribeSync(cfg.GigPublishedSubject)
		Expect(err).NotTo(HaveOccurred())
		Expect(rawConn.Flush()).To(Succeed())

		Expect(brokerAny.Publish(context.Background(), cfg.GigPublishedSubject, []byte("payload"))).To(Succeed())
		msg, err := sub.NextMsg(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(msg.Data)).To(Equal("payload"))

		received := make(chan []byte, 1)
		Expect(brokerAny.Subscribe(context.Background(), cfg.GigPublishedSubject, func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal(cfg.GigPublishedSubject))
			received <- payload
			return nil
		})).To(Succeed())

		Expect(rawConn.Publish(cfg.GigPublishedSubject, []byte("payload-2"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())
		Eventually(received).Should(Receive(Equal([]byte("payload-2"))))
	})

	It("runs a pull consumer", func() {
		brokerAny, err := NewBroker(cfg, brokerIntegrationLogger)
		Expect(err).NotTo(HaveOccurred())
		concrete := brokerAny.(*natsBroker)
		defer concrete.Close()

		var handled atomic.Int32
		Expect(concrete.RunPullConsumer(context.Background(), config.PullConsumerConfig{
			Stream:     cfg.GigEventsStream,
			Subject:    cfg.GigPublishedSubject,
			Durable:    cfg.GigProjectionDurable,
			BatchSize:  cfg.GigProjectionBatchSize,
			MaxWait:    cfg.GigProjectionMaxWait,
			Workers:    cfg.GigProjectionWorkers,
			QueueSize:  cfg.GigProjectionQueueSize,
			AckWait:    cfg.GigProjectionAckWait,
			MaxDeliver: cfg.GigProjectionMaxDeliver,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				CheckInterval:   cfg.GigProjectionAdaptiveCheckInterval,
				MediumPending:   cfg.GigProjectionAdaptiveMediumPending,
				HighPending:     cfg.GigProjectionAdaptiveHighPending,
				LowBatchSize:    cfg.GigProjectionAdaptiveLowBatchSize,
				LowMaxWait:      cfg.GigProjectionAdaptiveLowMaxWait,
				MediumBatchSize: cfg.GigProjectionAdaptiveMediumBatchSize,
				MediumMaxWait:   cfg.GigProjectionAdaptiveMediumMaxWait,
				HighBatchSize:   cfg.GigProjectionAdaptiveHighBatchSize,
				HighMaxWait:     cfg.GigProjectionAdaptiveHighMaxWait,
			},
		}, func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal(cfg.GigPublishedSubject))
			Expect(payload).NotTo(BeEmpty())
			handled.Add(1)
			return nil
		})).To(Succeed())

		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		Expect(rawConn.Publish(cfg.GigPublishedSubject, []byte("one"))).To(Succeed())
		Expect(rawConn.Publish(cfg.GigPublishedSubject, []byte("two"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		Eventually(func() int32 { return handled.Load() }).Should(Equal(int32(2)))
	})

	It("updates adaptive pull plans from pending messages", func() {
		rawConn, err := nats.Connect(brokerIntegrationURL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		cfg := config.PullConsumerConfig{
			Stream:     "GIG_EVENTS",
			Subject:    "gig.published",
			Durable:    "gig_projection_adaptive",
			BatchSize:  1,
			MaxWait:    10 * time.Millisecond,
			Workers:    1,
			QueueSize:  1,
			AckWait:    time.Second,
			MaxDeliver: 3,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				CheckInterval:   time.Millisecond,
				MediumPending:   1,
				HighPending:     2,
				LowBatchSize:    1,
				LowMaxWait:      2 * time.Millisecond,
				MediumBatchSize: 2,
				MediumMaxWait:   3 * time.Millisecond,
				HighBatchSize:   3,
				HighMaxWait:     4 * time.Millisecond,
			},
		}

		runtimeAny, err := newPullConsumerRuntimeFactory().Create(rawConn, brokerIntegrationLogger, cfg, func(context.Context, string, []byte) error {
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		runtime := runtimeAny.(*pullConsumerRuntime)

		Expect(rawConn.Publish(cfg.Subject, []byte("one"))).To(Succeed())
		Expect(rawConn.Publish(cfg.Subject, []byte("two"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		state := pullConsumerFetchState{
			batch:             cfg.BatchSize,
			wait:              cfg.MaxWait,
			tier:              "base",
			lastAdaptiveCheck: time.Now().Add(-time.Hour),
		}

		runtime.maybeUpdateAdaptivePlan(&state)
		Expect(state.tier).To(Equal("high"))
		Expect(state.batch).To(Equal(3))
		Expect(state.wait).To(Equal(4 * time.Millisecond))
	})
})
