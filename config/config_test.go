package config

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gig Config Helpers Suite")
}

var _ = Describe("Config", func() {
	It("loads configuration from the environment", func() {
		GinkgoT().Setenv("APP_ENV", "test")
		GinkgoT().Setenv("LOG_LEVEL", "debug")
		GinkgoT().Setenv("DB_HOST", "db")
		GinkgoT().Setenv("DB_PORT", "5433")
		GinkgoT().Setenv("DB_USER", "gig")
		GinkgoT().Setenv("DB_PASSWORD", "secret")
		GinkgoT().Setenv("DB_NAME", "gig_service")
		GinkgoT().Setenv("GRPC_HOST", "127.0.0.1")
		GinkgoT().Setenv("GRPC_PORT", "9503")
		GinkgoT().Setenv("REDIS_HOST", "redis")
		GinkgoT().Setenv("REDIS_PORT", "6380")
		GinkgoT().Setenv("NATS_URL", "nats://127.0.0.1:4222")
		GinkgoT().Setenv("FILE_SERVICE_ADDRESS", "127.0.0.1:9504")
		GinkgoT().Setenv("PAYMENT_SERVICE_ADDRESS", "127.0.0.1:9505")
		GinkgoT().Setenv("USER_SERVICE_ADDRESS", "127.0.0.1:9506")
		GinkgoT().Setenv("CLICKHOUSE_ENDPOINT", "http://127.0.0.1:8123")
		GinkgoT().Setenv("CLICKHOUSE_USER", "admin")
		GinkgoT().Setenv("CLICKHOUSE_PASSWORD", "admin")
		GinkgoT().Setenv("GIG_PREVIEW_PAGE_SIZE", "1")
		GinkgoT().Setenv("GIG_PREVIEW_WINDOW_SIZE", "2")
		GinkgoT().Setenv("GIG_PREVIEW_WINDOW_TTL", "15m")

		cfg, err := Load()

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.App.Env).To(Equal("test"))
		Expect(cfg.App.LogLevel).To(Equal("debug"))
		Expect(cfg.DB.Host).To(Equal("db"))
		Expect(cfg.GRPC.Port).To(Equal(9503))
		Expect(cfg.Redis.Port).To(Equal(6380))
		Expect(cfg.FileService.Address).To(Equal("127.0.0.1:9504"))
		Expect(cfg.Preview.PageSize).To(Equal(1))
		Expect(cfg.Preview.WindowSize).To(Equal(2))
	})

	It("wraps parsing errors", func() {
		GinkgoT().Setenv("APP_ENV", "test")
		GinkgoT().Setenv("LOG_LEVEL", "debug")
		GinkgoT().Setenv("DB_HOST", "db")
		GinkgoT().Setenv("DB_PORT", "not-a-number")
		GinkgoT().Setenv("DB_USER", "gig")
		GinkgoT().Setenv("DB_PASSWORD", "secret")
		GinkgoT().Setenv("DB_NAME", "gig_service")
		GinkgoT().Setenv("GRPC_HOST", "127.0.0.1")
		GinkgoT().Setenv("GRPC_PORT", "9503")
		GinkgoT().Setenv("REDIS_HOST", "redis")
		GinkgoT().Setenv("REDIS_PORT", "6380")
		GinkgoT().Setenv("NATS_URL", "nats://127.0.0.1:4222")
		GinkgoT().Setenv("FILE_SERVICE_ADDRESS", "127.0.0.1:9504")
		GinkgoT().Setenv("PAYMENT_SERVICE_ADDRESS", "127.0.0.1:9505")
		GinkgoT().Setenv("USER_SERVICE_ADDRESS", "127.0.0.1:9506")
		GinkgoT().Setenv("CLICKHOUSE_ENDPOINT", "http://127.0.0.1:8123")
		GinkgoT().Setenv("CLICKHOUSE_USER", "admin")
		GinkgoT().Setenv("CLICKHOUSE_PASSWORD", "admin")
		GinkgoT().Setenv("GIG_PREVIEW_PAGE_SIZE", "1")
		GinkgoT().Setenv("GIG_PREVIEW_WINDOW_SIZE", "2")
		GinkgoT().Setenv("GIG_PREVIEW_WINDOW_TTL", "15m")

		cfg, err := Load()

		Expect(cfg).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("parse env config")))
	})

	It("wraps parse errors directly", func() {
		err := WrapParseEnvConfigError(errors.New("boom"))
		Expect(err).To(MatchError(ContainSubstring("parse env config")))
	})
})
