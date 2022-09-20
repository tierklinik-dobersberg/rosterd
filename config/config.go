package config

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type (
	ServiceConfig struct {
		// IdentityProvider holds the address of the identity provider.
		IdentityProvider string `env:"IDENTITY_PROVIDER,required"`
		// JWTSecret holds the secret to validate JWTs issued by CIS.
		JWTSecret string `env:"JWT_SECRET,required"`
		// DatabaseURL is the mongodb connection URL
		DatabaseURL string `env:"DATABASE_URL,required"`
		// DatabaseName is the name of the mongodb database.
		DatabaseName string `env:"DATABASE_NAME,required"`
		// AdminRoles defines the roles that are allowed to create
		// and manage the rosters.
		AdminRoles []string `env:"ADMIN_ROLES"`
		// Address holds the listen address of the HTTP server.
		Address string `env:"ADDRESS,default=:8080"`
		// Country is the two-letter country code of legal residence used
		// for public holiday detection.
		Country string `env:"COUNTRY,default=AT"`
	}
)

// Read reads the service configuration from environment variables
func Read(ctx context.Context) (*ServiceConfig, error) {
	var cfg ServiceConfig

	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
