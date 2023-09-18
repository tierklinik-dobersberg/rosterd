package config

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1/calendarv1connect"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1/idmv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/overlayfs"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type Providers struct {
	Users     idmv1connect.UserServiceClient
	Roles     idmv1connect.RoleServiceClient
	Notify    idmv1connect.NotifyServiceClient
	Calendar  calendarv1connect.CalendarServiceClient
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

	db, err := database.NewDatabase(
		ctx,
		mongoClient.Database(cfg.DatabaseName),
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

func (p *Providers) FetchAuthUserRoles(ctx context.Context, req connect.AnyRequest) ([]*idmv1.Role, error) {
	roleIds := req.Header().Values("X-Remote-Role")

	roles := make([]*idmv1.Role, len(roleIds))
	for idx, roleId := range roleIds {
		res, err := p.Roles.GetRole(ctx, connect.NewRequest(&idmv1.GetRoleRequest{
			Search: &idmv1.GetRoleRequest_Id{
				Id: roleId,
			},
		}))

		if err != nil {
			return nil, err
		}

		roles[idx] = res.Msg.Role
	}

	return roles, nil
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
			Paths: []string{"users.user.id", "users.user.username", "users.roles", "users.user.primary_mail", "users.user.display_name"},
		},
	}))
	if err != nil {
		return nil, err
	}

	return res.Msg.Users, nil
}
