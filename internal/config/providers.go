package config

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1/calendarv1connect"
	eventsv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1/eventsv1connect"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1/idmv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"github.com/tierklinik-dobersberg/apis/pkg/overlayfs"
	"github.com/tierklinik-dobersberg/rosterd/internal/database"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/dcaraxes/gotenberg-go-client/v8"
	"github.com/dcaraxes/gotenberg-go-client/v8/document"
)

type Providers struct {
	Users     idmv1connect.UserServiceClient
	Roles     idmv1connect.RoleServiceClient
	Notify    idmv1connect.NotifyServiceClient
	Calendar  calendarv1connect.CalendarServiceClient
	Events    eventsv1connect.EventServiceClient
	Holidays  calendarv1connect.HolidayServiceClient
	Templates fs.FS
	Datastore *database.DatabaseImpl
	Config    *ServiceConfig
}

func NewProviders(ctx context.Context, cfg *ServiceConfig, httpClient *http.Client, template embed.FS) (*Providers, error) {
	fileSystems := []fs.FS{template}

	if cfg.TemplatesDir != "" {
		overwrite := os.DirFS(cfg.TemplatesDir)

		fileSystems = []fs.FS{
			overwrite,
			template,
		}
	}

	clientOptions := options.Client().
		ApplyURI(cfg.DatabaseURL).
		SetAppName("rosterd")

	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongodb client: %w", err)
	}

	if err := mongoClient.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	mongoDatabase := mongoClient.Database(cfg.DatabaseName)

	// before doing anything more, let's migrate our database
	if err := database.RunMigrations(ctx, mongoDatabase); err != nil {
		slog.Error("failed to run migrations", "error", err.Error())

		// FIXME(ppacher): do not ignore this error here.
	}

	// finally, create our repository (database wrapper)
	db, err := database.NewDatabase(
		ctx,
		mongoDatabase,
		logrus.NewEntry(logrus.StandardLogger()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to perpare database: %w", err)
	}

	p := &Providers{
		Config:    cfg,
		Users:     idmv1connect.NewUserServiceClient(httpClient, cfg.IdentityProvider),
		Roles:     idmv1connect.NewRoleServiceClient(httpClient, cfg.IdentityProvider),
		Notify:    idmv1connect.NewNotifyServiceClient(httpClient, cfg.IdentityProvider),
		Calendar:  calendarv1connect.NewCalendarServiceClient(httpClient, cfg.CalendarService),
		Holidays:  calendarv1connect.NewHolidayServiceClient(httpClient, cfg.CalendarService),
		Events:    eventsv1connect.NewEventServiceClient(cli.NewInsecureHttp2Client(), cfg.EventServiceUrl),
		Templates: overlayfs.NewFS(fileSystems...),
		Datastore: db,
	}

	return p, nil
}

func (p *Providers) FetchAllUserIds(ctx context.Context) ([]string, error) {
	allUsers, err := p.Users.ListUsers(ctx, connect.NewRequest(&idmv1.ListUsersRequest{
		FieldMask: &fieldmaskpb.FieldMask{
			Paths: []string{"users.user.id"},
		},
	}))

	if err != nil {
		return nil, fmt.Errorf("failed to fetch Users: %w", err)
	}

	userIds := make([]string, len(allUsers.Msg.Users))
	for idx, u := range allUsers.Msg.Users {
		userIds[idx] = u.User.Id
	}

	return userIds, nil
}

func (p *Providers) VerifyUserExists(ctx context.Context, id string) error {
	_, err := p.Users.GetUser(ctx, connect.NewRequest(&idmv1.GetUserRequest{
		Search: &idmv1.GetUserRequest_Id{
			Id: id,
		},
		FieldMask: &fieldmaskpb.FieldMask{
			Paths: []string{"profile.user.id"},
		},
	}))

	return err
}

func (p *Providers) FetchAllUserProfiles(ctx context.Context) ([]*idmv1.Profile, error) {
	res, err := p.Users.ListUsers(ctx, connect.NewRequest(&idmv1.ListUsersRequest{
		FieldMask: &fieldmaskpb.FieldMask{
			Paths: []string{"users.user.avatar"},
		},
		ExcludeFields: true,
	}))
	if err != nil {
		return nil, err
	}

	return res.Msg.Users, nil
}

func (p *Providers) RenderHTML(ctx context.Context, index string) (io.ReadCloser, error) {
	if p.Config.Gotenberg == "" {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("no gotenberg server configured"))
	}

	client, err := gotenberg.NewClient(p.Config.Gotenberg, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create gotenberg client: %w", err)
	}

	indexDocument, err := document.FromString("index.html", index)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare document: %w", err)
	}

	req := gotenberg.NewHTMLRequest(indexDocument)
	req.PaperSize(gotenberg.A4)
	req.Landscape()
	req.Margins(gotenberg.NoMargins)
	req.SkipNetworkIdleEvent()
	req.WaitDelay(time.Second * 3)
	req.Scale(0.75)

	res, err := client.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to gotenberg: %w", err)
	}

	if res.StatusCode != 200 {
		res.Body.Close()

		return nil, fmt.Errorf("unexpected response from gotenberg: %s", res.Status)
	}

	return res.Body, nil
}

func (p *Providers) PublishEvent(msg proto.Message, retained bool) {
	go func() {
		pb, err := anypb.New(msg)
		if err != nil {
			slog.Error("failed to marshal protobuf message as anypb.Any", "error", err, "messageType", proto.MessageName(msg))
			return
		}

		evt := &eventsv1.Event{
			Event:    pb,
			Retained: retained,
		}

		if _, err := p.Events.Publish(context.Background(), connect.NewRequest(evt)); err != nil {
			slog.Error("failed to publish event", "error", err, "messageType", proto.MessageName(msg))
		}
	}()
}
