package framework

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/holiday"
	"github.com/tierklinik-dobersberg/rosterd/server"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoContainer struct {
	testcontainers.Container

	URI    string
	Client *mongo.Client
}

type Environment struct {
	Identitiy *MockIdentityProvider
	Mongo     mongoContainer
}

func (env *Environment) Teardown(ctx context.Context) error {
	merr := new(multierror.Error)

	if err := env.Mongo.Terminate(ctx); err != nil {
		merr.Errors = append(merr.Errors, fmt.Errorf("failed to terminate mongodb: %w", err))
	}

	if err := env.Mongo.Client.Disconnect(ctx); err != nil {
		merr.Errors = append(merr.Errors, fmt.Errorf("failed to disconnect from mongo: %w", err))
	}

	return merr.ErrorOrNil()
}

func SetupEnvironment(ctx context.Context, t *testing.T) (*Environment, error) {

	req := testcontainers.ContainerRequest{
		Image: "mongo:latest",
		ExposedPorts: []string{
			"27017/tcp",
		},
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "root",
			"MONGO_INITDB_ROOT_PASSWORD": "example",
		},
		WaitingFor: wait.ForExposedPort(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	mongoIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedMongoPort, err := container.MappedPort(ctx, "27017")
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("mongodb://root:example@%s:%s/", mongoIP, mappedMongoPort.Port())

	opts := options.Client().ApplyURI(uri)
	cli, err := mongo.NewClient(opts)
	if err != nil {
		container.Terminate(ctx)

		return nil, err
	}

	if err := cli.Connect(ctx); err != nil {
		container.Terminate(ctx)

		return nil, err
	}

	dbName := namesgenerator.GetRandomName(0)
	mongoDB := cli.Database(dbName)

	logger := hclog.L()
	db, err := database.NewDatabase(ctx, mongoDB, logger.Named("database"))
	if err != nil {
		container.Terminate(ctx)

		return nil, err
	}

	secret := "secret"

	identityProvider := NewMockIdentityProvider(secret)

	srv := server.Server{
		Database:         db,
		IdentityProvider: identityProvider,
		JWTSecret:        secret,
		Logger:           logger.Named("server"),
		AdminRoles:       []string{"admin"},
		Address:          "127.0.0.1:12345",
		Holidays:         holiday.NewHolidayCache(logger.Named("holiday")),
		Country:          "AT",
	}

	if err := srv.Setup(); err != nil {
		container.Terminate(ctx)

		return nil, err
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			t.Errorf("failed to listen: %w", err)
			t.FailNow()
		}
	}()

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	return &Environment{
		Identitiy: identityProvider,
		Mongo: mongoContainer{
			Container: container,
			URI:       uri,
			Client:    cli,
		},
	}, nil
}
