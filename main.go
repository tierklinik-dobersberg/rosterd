package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/cors"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/apis/pkg/privacy"
	apisrv "github.com/tierklinik-dobersberg/apis/pkg/server"
	"github.com/tierklinik-dobersberg/apis/pkg/spa"
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/services/offtime"
	"github.com/tierklinik-dobersberg/rosterd/services/roster"
	"github.com/tierklinik-dobersberg/rosterd/services/workshift"
	"github.com/tierklinik-dobersberg/rosterd/services/worktime"
	"google.golang.org/protobuf/reflect/protoregistry"
)

//go:embed ui/dist/ui
var static embed.FS

//go:embed mails/dist
var mailTemplates embed.FS

func main() {
	ctx := context.Background()

	l := logrus.StandardLogger()

	cfg, err := config.Read(ctx)
	if err != nil {
		l.Error("failed to read configuration", "error", err)
		os.Exit(1)
	}

	p, err := config.NewProviders(ctx, cfg, http.DefaultClient, mailTemplates)
	if err != nil {
		l.Error("failed to create application providers: %w", err)
		os.Exit(1)
	}

	if err := bootstrapRosterManagerRole(p); err != nil {
		logrus.Fatalf("failed to bootstrap roster_manager role: %s", err)
	}

	/*
		location, err := time.LoadLocation("Europe/Vienna")
		if err != nil {
			l.Error("failed to load location data", "error", err.Error())
			os.Exit(1)
		}
	*/

	publicServer, adminServer := prepareConnectServer(p)

	if err := apisrv.Serve(context.Background(), publicServer, adminServer); err != nil {
		logrus.Fatalf("failed to serve: %s", err)
	}
}

func bootstrapRosterManagerRole(p *config.Providers) error {
	// make sure there's a roster_manager role available
	getRoleRes, err := p.Roles.GetRole(context.Background(), connect.NewRequest(&idmv1.GetRoleRequest{
		Search: &idmv1.GetRoleRequest_Name{
			Name: "roster_manager",
		},
	}))

	if err != nil {
		var cerr *connect.Error

		if errors.As(err, &cerr) && cerr.Code() == connect.CodeNotFound {
			createRoleRes, err := p.Roles.CreateRole(context.Background(), connect.NewRequest(&idmv1.CreateRoleRequest{
				Name:             "roster_manager",
				Description:      "Administration role for rosters",
				DeleteProtection: true,
			}))

			if err != nil {
				return fmt.Errorf("failed to create role: %w", err)
			}

			// TODO(ppacher): automatically add all idm_superusers to the roster_manager role.

			if p.Config.RosterManagerRoleID == "" {
				p.Config.RosterManagerRoleID = createRoleRes.Msg.Role.Id
			}

			logrus.Infof("created roster_manager role: id=%q", createRoleRes.Msg.Role.Id)
		} else {
			return fmt.Errorf("failed to get role: %w", err)
		}
	} else {
		logrus.Infof("found roster_manager role in IDM: id=%q", getRoleRes.Msg.GetRole().GetId())

		if p.Config.RosterManagerRoleID == "" {
			p.Config.RosterManagerRoleID = getRoleRes.Msg.Role.Id
		}
	}

	return nil
}

func getStaticFilesHandler(path string) (http.Handler, error) {
	if path == "" {
		webapp, err := fs.Sub(static, "ui/dist/ui")
		if err != nil {
			return nil, err
		}
		return spa.ServeSPA(http.FS(webapp), "index.html"), nil
	}

	if strings.HasPrefix(path, "http") {
		remote, err := url.Parse(path)
		if err != nil {
			return nil, err
		}

		handler := func(p *httputil.ReverseProxy) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Host = remote.Host
				p.ServeHTTP(w, r)
			})
		}

		return handler(httputil.NewSingleHostReverseProxy(remote)), nil
	}

	return spa.ServeSPA(http.Dir(path), "index.html"), nil
}

var serverContextKey = struct{ S string }{S: "serverContextKey"}

func prepareConnectServer(p *config.Providers) (public, admin *http.Server) {
	privacyInterceptor := privacy.NewFilterInterceptor(privacy.SubjectResolverFunc(func(ctx context.Context, ar connect.AnyRequest) (string, []string, error) {
		remoteUser := auth.From(ctx)

		if remoteUser == nil {
			return "", nil, nil
		}

		return remoteUser.ID, remoteUser.RoleIDs, nil
	}))

	logInterceptor := log.NewLoggingInterceptor()

	authInterceptor := auth.NewAuthAnnotationInterceptor(protoregistry.GlobalFiles, auth.NewIDMRoleResolver(p.Roles), func(ctx context.Context, req connect.AnyRequest) (auth.RemoteUser, error) {
		serverKey, _ := ctx.Value(serverContextKey).(string)

		if serverKey == "admin" {
			return auth.RemoteUser{
				ID:          "service-account",
				DisplayName: req.Peer().Addr,
				RoleIDs:     []string{p.Config.RosterManagerRoleID},
				Admin:       true,
			}, nil
		}

		return auth.RemoteHeaderExtractor(ctx, req)
	})

	interceptors := connect.WithInterceptors(
		authInterceptor,
		privacyInterceptor,
		logInterceptor,
	)

	mux := http.NewServeMux()

	workTimeService := worktime.New(p)
	path, handler := rosterv1connect.NewWorkTimeServiceHandler(workTimeService, interceptors)
	mux.Handle(path, handler)

	workShiftService := workshift.New(p)
	path, handler = rosterv1connect.NewWorkShiftServiceHandler(workShiftService, interceptors)
	mux.Handle(path, handler)

	offTimeService := offtime.New(p)
	path, handler = rosterv1connect.NewOffTimeServiceHandler(offTimeService, interceptors)
	mux.Handle(path, handler)

	rosterService := roster.NewRosterService(p)
	path, handler = rosterv1connect.NewRosterServiceHandler(rosterService, interceptors)
	mux.Handle(path, handler)

	constraintService := roster.NewConstraintService(p)
	path, handler = rosterv1connect.NewConstraintServiceHandler(constraintService, interceptors)
	mux.Handle(path, handler)

	// Get a static file handler.
	// This will either return a handler for the embed.FS, a local directory using http.Dir
	// or a reverse proxy to some other service.
	staticFilesHandler, err := getStaticFilesHandler(os.Getenv("STATIC_DIR"))
	if err != nil {
		logrus.Fatal(err)
	}

	mux.Handle("/", staticFilesHandler)

	cfg := cors.Config{
		AllowedOrigins:   p.Config.AllowedOrigins,
		AllowCredentials: true,
		Debug:            true,
	}

	wrapWithKey := func(key string, next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), serverContextKey, key))

			next.ServeHTTP(w, r)
		})
	}

	public = apisrv.Create(p.Config.Address, cors.Wrap(cfg, wrapWithKey("public", mux)))
	admin = apisrv.Create(p.Config.AdminAddress, wrapWithKey("admin", mux))

	return public, admin
}
