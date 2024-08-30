package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type (
	ServiceConfig struct {
		// IdentityProvider holds the address of the identity provider.
		IdentityProvider string `env:"IDM_URL,default=http://cisidm:8081"`
		// DatabaseURL is the mongodb connection URL
		DatabaseURL string `env:"DATABASE_URL,required"`
		// DatabaseName is the name of the mongodb database.
		DatabaseName string `env:"DATABASE_NAME,required"`
		// Address holds the listen address of the HTTP server.
		Address string `env:"ADDRESS,default=:8080"`
		// AdminAddress holds the address of the unauthenticated admin endpoint.
		AdminAddress string `env:"ADMIN_ADDRESS,default=:8081"`
		// Country is the two-letter country code of legal residence used
		// for public holiday detection.
		Country string `env:"COUNTRY,default=AT"`
		// CalendarServiceURL holds the URL of the calendar service.
		CalendarService string `env:"CALENDAR_SERVICE_URL,default=http://ciscal:8080"`
		// PublicURL is the public URL to rosterd
		PublicURL string `env:"PUBLIC_URL"`
		// PreviewRosterURL should be set to the format string accepting year and month
		// (in this order) to build a public link to access a readonly version of the roster.
		// If left empty, this defaults to {{ PublicURL }}/roster/view/%s
		PreviewRosterURL string `env:"PREVIEW_ROSTER_URL"`
		// TemplatesDir might be set to a directory path containing mail and SMS template
		// files. If set, any files in TemplateDir will overwrite the embedded templates
		// of the final rosterd binary.
		TemplatesDir string `env:"TEMPLATES_PATH"`
		// AllowedOrigins configures the allowed CORS domains.
		AllowedOrigins []string `env:"ALLOWED_ORIGINS"`
		// RosterManagerRoleID holds the ID of the roster_manager role
		RosterManagerRoleID string `env:"ROSTER_MANAGER_ROLE_ID"`
		// Path or URL for the rosterd frontend
		StaticFiles string `env:"STATIC_FILES"`
		// Gotenberg holds the gotenberg URL
		Gotenberg string `env:"GOTENBERG"`
		// EventServiceUrl holds the URL of the event-service used to publish
		// messages.
		EventServiceUrl string `env:"EVENTS_SERVICE_URL,required"`
	}
)

// Read reads the service configuration from environment variables
func Read(ctx context.Context) (*ServiceConfig, error) {
	var cfg ServiceConfig

	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, err
	}

	if cfg.PublicURL == "" {
		return &cfg, fmt.Errorf("missing PUBLIC_URL configuration")
	}

	if cfg.PreviewRosterURL == "" {
		cfg.PreviewRosterURL = fmt.Sprintf("%s/roster/view/%%s", cfg.PublicURL)
	}

	return &cfg, nil
}
