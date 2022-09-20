package main

import (
	"context"
	"math/rand"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/holiday"
	"github.com/tierklinik-dobersberg/rosterd/identity"
	"github.com/tierklinik-dobersberg/rosterd/server"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx := context.Background()

	rand.Seed(time.Now().UnixNano())

	l := hclog.Default().Named("rosterd")

	cfg, err := config.Read(ctx)
	if err != nil {
		l.Error("failed to read configuration", "error", err)
		os.Exit(1)
	}

	clientOptions := options.Client().
		ApplyURI(cfg.DatabaseURL).
		SetAppName("rosterd")

	mongoClient, err := mongo.NewClient(clientOptions)
	if err != nil {
		l.Error("failed to create mongodb client", "error", err)
		os.Exit(1)
	}

	if err := mongoClient.Connect(ctx); err != nil {
		l.Error("failed to connect to mongodb", "error", err)
		os.Exit(1)
	}

	db, err := database.NewDatabase(
		ctx,
		mongoClient.Database(cfg.DatabaseName),
		l.Named("database"),
	)
	if err != nil {
		l.Error("failed to create database", "error", err)
		os.Exit(1)
	}

	var identityProvider identity.Provider
	if _, err := os.Stat(cfg.IdentityProvider); err == nil {
		identityProvider, err = identity.NewFileProvider(cfg.IdentityProvider)
		if err != nil {
			l.Error("invalid identity provider configuration")
			os.Exit(1)
		}
	} else {
		identityProvider = &identity.HTTPProvider{
			BaseURL: cfg.IdentityProvider,
			Client:  retryablehttp.NewClient(),
		}
	}

	srv := server.Server{
		Database:         db,
		IdentityProvider: identityProvider,
		JWTSecret:        cfg.JWTSecret,
		Logger:           l.Named("server"),
		AdminRoles:       cfg.AdminRoles,
		Address:          cfg.Address,
		Holidays:         holiday.NewHolidayCache(l.Named("holiday")),
		Country:          cfg.Country,
	}

	if err := srv.Setup(); err != nil {
		l.Error("failed to setup HTTP server", "error", err)
		os.Exit(1)
	}

	if err := srv.ListenAndServe(); err != nil {
		l.Error("failed to listen", "address", srv.Address, "error", err)
		os.Exit(1)
	}
}
