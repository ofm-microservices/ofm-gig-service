package config

// FileServiceConfig defines the upstream file-service connection used when gig
// media uploads need to be persisted as file IDs.
type FileServiceConfig struct {
	Address string `env:"FILE_SERVICE_ADDRESS,required"`
}
