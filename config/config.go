package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type (
	ServiceConfig struct {
		// IdentityProvider holds the address of the identity provider.
		IdentityProvider string `env:"IDENTITY_PROVIDER"`
		// JWTSecret holds the secret to validate JWTs issued by CIS.
		JWTSecret string `env:"JWT_SECRET,required"`
		// DatabaseURL is the mongodb connection URL
		DatabaseURL string `env:"DATABASE_URL,required"`
		// DatabaseName is the name of the mongodb database.
		DatabaseName string `env:"DATABASE_NAME,required"`
		// AdminRoles defines the roles that are allowed to create
		// and manage the rosters.
		AdminRoles []string `env:"ADMIN_ROLES"`
		// Standalone may be set during development. In this mode, rosterd
		// will not load available identities from the identity provider but rather
		// accept all identities without verification.
		Standalone bool `env:"STANDALONE"`
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

	if !cfg.Standalone && cfg.IdentityProvider == "" {
		return nil, fmt.Errorf("missing either STANDALONE or IDENTITY_PROVIDER environment variable")
	}

	return &cfg, nil
}
