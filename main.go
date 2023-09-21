package main

import (
	"context"
	"embed"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
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
)

//go:embed ui/dist/ui
var static embed.FS

//go:embed mails/dist
var mailTemplates embed.FS

func main() {
	ctx := context.Background()

	rand.Seed(time.Now().UnixNano())

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

	/*
		location, err := time.LoadLocation("Europe/Vienna")
		if err != nil {
			l.Error("failed to load location data", "error", err.Error())
			os.Exit(1)
		}
	*/

	connectSrv := prepareConnectServer(p)

	if err := apisrv.Serve(context.Background(), connectSrv); err != nil {
		logrus.Fatalf("failed to serve: %s", err)
	}
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

func prepareConnectServer(p *config.Providers) *http.Server {
	privacyInterceptor := privacy.NewFilterInterceptor(privacy.SubjectResolverFunc(func(ctx context.Context, ar connect.AnyRequest) (string, []string, error) {
		userId := ar.Header().Get("X-Remote-User-ID")
		roles := ar.Header().Values("X-Remote-Role")

		return userId, roles, nil
	}))

	logInterceptor := log.NewLoggingInterceptor()

	interceptors := connect.WithInterceptors(
		logInterceptor,
		privacyInterceptor,
	)

	mux := http.NewServeMux()

	workTimeService := worktime.New(p)
	path, handler := rosterv1connect.NewWorkTimeServiceHandler(workTimeService, interceptors)
	mux.Handle(path, handler)

	workShiftService := workshift.New(p)
	path, handler = rosterv1connect.NewWorkShiftServiceHandler(workShiftService)
	mux.Handle(path, handler)

	offTimeService := offtime.New(p)
	path, handler = rosterv1connect.NewOffTimeServiceHandler(offTimeService, interceptors)
	mux.Handle(path, handler)

	rosterService := roster.NewRosterService(p)
	path, handler = rosterv1connect.NewRosterServiceHandler(rosterService)
	mux.Handle(path, handler)

	constraintService := roster.NewConstraintService(p)
	path, handler = rosterv1connect.NewConstraintServiceHandler(constraintService)
	mux.Handle(path, handler)

	// Get a static file handler.
	// This will either return a handler for the embed.FS, a local directory using http.Dir
	// or a reverse proxy to some other service.
	staticFilesHandler, err := getStaticFilesHandler("")
	if err != nil {
		logrus.Fatal(err)
	}

	mux.Handle("/", staticFilesHandler)

	cfg := cors.Config{
		AllowedOrigins: []string{
			"http://*.dobersberg.dev",
			"https://*.dobersberg.dev",
			"http://localhost:5000",
		},
		AllowCredentials: true,
	}

	return apisrv.Create(p.Config.Address, cors.Wrap(cfg, mux))
}
