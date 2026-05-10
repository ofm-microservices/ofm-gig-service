package nats

import (
	"errors"

	"gig-service/config"
	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeBootstrapConn struct {
	js  jetStreamManager
	err error
}

func (f fakeBootstrapConn) JetStream() (jetStreamManager, error) { return f.js, f.err }
func (fakeBootstrapConn) Close()                                 {}

type fakeJetStream struct {
	addErr    error
	updateErr error
	addCalls  int
	updateCalls int
}

func (f *fakeJetStream) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.addCalls++
	if f.addErr != nil {
		return nil, f.addErr
	}
	return &nats.StreamInfo{}, nil
}

func (f *fakeJetStream) UpdateStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.updateCalls++
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &nats.StreamInfo{}, nil
}

var _ = Describe("nats bootstrap", func() {
	var logger logging.Logger

	BeforeEach(func() {
		var err error
		logger, err = logging.New("gig-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	It("connects to nats and wraps failures", func() {
		conn, err := Connect(config.NATSConfig{URL: "nats://127.0.0.1:1"})
		Expect(conn).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})

	It("ensures streams using the configured bootstrap connection", func() {
		original := connectBootstrap
		DeferCleanup(func() { connectBootstrap = original })

		addErr := errors.New("add failed")
		updateErr := errors.New("update failed")
		js := &fakeJetStream{addErr: addErr, updateErr: nil}
		connectBootstrap = func(config.NATSConfig) (bootstrapConn, error) {
			return fakeBootstrapConn{js: js}, nil
		}

		Expect(EnsureStream(config.NATSConfig{
			URL:                 "nats://unused",
			GigEventsStream:     "GIG_EVENTS",
			GigPublishedSubject: "gig.published",
		}, logger)).To(Succeed())
		Expect(js.addCalls).To(Equal(1))

		js = &fakeJetStream{addErr: addErr, updateErr: updateErr}
		connectBootstrap = func(config.NATSConfig) (bootstrapConn, error) {
			return fakeBootstrapConn{js: js}, nil
		}
		Expect(EnsureStream(config.NATSConfig{
			URL:                 "nats://unused",
			GigEventsStream:     "GIG_EVENTS",
			GigPublishedSubject: "gig.published",
		}, logger)).To(MatchError(ContainSubstring("ensure stream")))

		connectBootstrap = func(config.NATSConfig) (bootstrapConn, error) {
			return nil, errors.New("connect failed")
		}
		Expect(EnsureStream(config.NATSConfig{URL: "nats://unused"}, logger)).To(HaveOccurred())
	})

	It("wraps bootstrap errors", func() {
		Expect(WrapInitJetStreamContextError(errors.New("boom"))).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureStreamError("STREAM", errors.New("add"), errors.New("update"))).To(MatchError(ContainSubstring("ensure stream")))
		Expect(WrapConnectToNATSError(errors.New("boom"))).To(MatchError(ContainSubstring("connect to nats")))
		Expect(ErrNilLogger).NotTo(BeNil())
	})
})
