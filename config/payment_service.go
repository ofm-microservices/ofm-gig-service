package config

// PaymentServiceConfig defines the upstream payment-service gRPC connection used
// to check Stripe Connect onboarding state before gig publication.
type PaymentServiceConfig struct {
	Address string `env:"PAYMENT_SERVICE_ADDRESS,required"`
}
