package nats

import (
	"context"
	"fmt"
	"time"

	"gig-service/config"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var natsBootstrapContainer testcontainers.Container
var natsBootstrapURL string
var natsBootstrapLogger logging.Logger

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var err error
	natsBootstrapLogger, err = logging.New("gig-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())

	natsBootstrapContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.12.4-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := natsBootstrapContainer.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := natsBootstrapContainer.MappedPort(ctx, "4222/tcp")
	Expect(err).NotTo(HaveOccurred())
	natsBootstrapURL = fmt.Sprintf("nats://%s:%s", host, port.Port())
})

var _ = AfterSuite(func() {
	if natsBootstrapContainer != nil {
		Expect(natsBootstrapContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("nats bootstrap integration", func() {
	It("connects and ensures the gig stream", func() {
		conn, err := Connect(config.NATSConfig{URL: natsBootstrapURL})
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close()

		Expect(EnsureStream(config.NATSConfig{
			URL:                 natsBootstrapURL,
			GigEventsStream:     "GIG_EVENTS",
			GigPublishedSubject: "gig.published",
			GigProjectionSubject:"gig.projection.requested",
		}, natsBootstrapLogger)).To(Succeed())

		js, err := conn.JetStream()
		Expect(err).NotTo(HaveOccurred())
		info, err := js.StreamInfo("GIG_EVENTS")
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Config.Subjects).To(ContainElement("gig.published"))
		Expect(info.Config.Subjects).To(ContainElement("gig.projection.requested"))
	})

	It("wraps invalid connection parameters", func() {
		conn, err := Connect(config.NATSConfig{URL: "nats://127.0.0.1:1"})
		Expect(conn).To(BeNil())
		Expect(err).To(HaveOccurred())
	})
})
