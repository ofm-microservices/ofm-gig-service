package redis

import (
	"context"
	"errors"
	"strconv"

	"github.com/alicebob/miniredis/v2"
	"gig-service/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("redis bootstrap", func() {
	It("opens a redis client and pings it", func() {
		srv, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		defer srv.Close()
		port, err := strconv.Atoi(srv.Port())
		Expect(err).NotTo(HaveOccurred())

		client, err := Open(context.Background(), config.RedisConfig{
			Host: "127.0.0.1",
			Port: port,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())
		Expect(client.Close()).To(Succeed())
	})

	It("wraps ping errors", func() {
		client, err := Open(context.Background(), config.RedisConfig{Host: "127.0.0.1", Port: 1})
		Expect(client).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ping redis"))
	})

	It("wraps ping failures directly", func() {
		Expect(WrapRedisPingError(errors.New("boom"))).To(MatchError(ContainSubstring("ping redis")))
	})
})
