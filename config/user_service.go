package config

// UserServiceConfig defines the upstream user-service gRPC connection used to
// resolve seller usernames during gig-service bootstrap.
type UserServiceConfig struct {
	Address string `env:"USER_SERVICE_ADDRESS,required"`
}
