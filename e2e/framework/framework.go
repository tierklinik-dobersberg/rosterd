package framework

import (
	"context"
	"flag"
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

type (
	mongoContainer struct {
		testcontainers.Container

		URI    string
		Client *mongo.Client
	}

	Environment struct {
		Identitiy *MockIdentityProvider
		Mongo     mongoContainer

		Database *mongo.Database
	}
)

// Test flags
var (
	mongoURIFlag = flag.String("mongo", "", "The MongoDB server URI. Leave empty to start a new container")
	keepDatabase = flag.Bool("keep-db", false, "Do not drop the test database. Only makes sense with -mongo")
)

func (m *mongoContainer) Terminate(ctx context.Context) error {
	if m.Container == nil {
		return nil
	}

	return m.Container.Terminate(ctx)
}

func (env *Environment) Teardown(ctx context.Context) error {
	merr := new(multierror.Error)

	if err := env.Mongo.Terminate(ctx); err != nil {
		merr.Errors = append(merr.Errors, fmt.Errorf("failed to terminate mongodb: %w", err))
	}

	if err := env.Mongo.Client.Disconnect(ctx); err != nil {
		merr.Errors = append(merr.Errors, fmt.Errorf("failed to disconnect from mongo: %w", err))
	}

	if !*keepDatabase {
		if err := env.Database.Drop(ctx); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("failed to drop test database: %w", err))
		}
	}

	return merr.ErrorOrNil()
}

func SetupEnvironment(ctx context.Context, t *testing.T) (env *Environment, err error) {
	var (
		uri       string
		container testcontainers.Container
	)

	if *mongoURIFlag == "" {
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

		container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				container.Terminate(ctx)
			}
		}()

		mongoIP, err := container.Host(ctx)
		if err != nil {
			return nil, err
		}

		mappedMongoPort, err := container.MappedPort(ctx, "27017")
		if err != nil {
			return nil, err
		}
		uri = fmt.Sprintf("mongodb://root:example@%s:%s/", mongoIP, mappedMongoPort.Port())
	} else {
		uri = *mongoURIFlag
	}

	opts := options.Client().ApplyURI(uri)
	cli, err := mongo.NewClient(opts)
	if err != nil {
		return nil, err
	}

	if err := cli.Connect(ctx); err != nil {
		return nil, err
	}

	dbName := namesgenerator.GetRandomName(0)
	mongoDB := cli.Database(dbName)

	logger := hclog.L()
	db, err := database.NewDatabase(ctx, mongoDB, logger.Named("database"))
	if err != nil {
		return nil, err
	}

	db.SetDebug(true)

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
		Database:  mongoDB,
		Mongo: mongoContainer{
			Container: container,
			URI:       uri,
			Client:    cli,
		},
	}, nil
}
