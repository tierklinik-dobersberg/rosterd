package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/holiday"
	"github.com/tierklinik-dobersberg/rosterd/identity"
	"github.com/tierklinik-dobersberg/rosterd/middleware"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/mongo"
)

type (
	HandlerFunc func(ctx context.Context, query url.Values, pathParam map[string]string, body io.Reader) (any, error)

	Server struct {
		// Database is the database implementation used to store, retrieve and
		// query work shifts and created rosters.
		Database interface {
			database.WorkShiftDatabase
			database.OffTimeDatabase
			database.ConstraintDatabase
			database.WorkTimeDatabase
		}

		// IdentityProvider provides access to all available identities.
		IdentityProvider identity.Provider

		// JWTSecret holds the secret key that is used by the JWT issuer
		//
		// TODO(ppacher): switch to a Pub/Private Key based verification
		JWTSecret string

		// Logger is the logger to use for requests.
		Logger hclog.Logger

		// AdminRoles defines a list of user roles that are allowed to manage
		// rosters and off-times.
		AdminRoles []string

		// Address holds the listen address of the server.
		Address string

		// Holidays is a getter to retrieve public holidays
		Holidays holiday.HolidayGetter

		// Country is the country of legal residence for which public
		// holidays should be loaded.
		Country string

		echo *echo.Echo
	}
)

func (srv *Server) ListenAndServe() error {
	return srv.echo.Start(srv.Address)
}

func (srv *Server) Setup() error {
	srv.echo = echo.New()

	srv.echo.Use(
		middleware.RequestLogger(srv.Logger),
		middleware.JWTAuth("cis-session", srv.JWTSecret),
	)

	v1 := srv.echo.Group("v1/")

	workShift := v1.Group("workshift")
	{
		workShift.GET("", wrap(srv.ListWorkShifts))
		workShift.POST("", wrap(srv.CreateWorkShift))
		workShift.PUT("/:id", wrap(srv.UpdateWorkShift))
		workShift.DELETE("/:id", wrap(srv.DeleteWorkShift))
	}

	roster := v1.Group("roster/")
	{
		roster.GET("shifts", wrap(srv.GetRequiredShifts))
		roster.POST("analyze", wrap(srv.AnalyzeRoster))
		roster.POST("generate/:year/:month", wrap(srv.GenerateRoster))
	}

	offTime := v1.Group("offtime/")
	{
		offTime.GET("", wrap(srv.FindOffTimeRequests))
		offTime.POST("", wrap(srv.CreateOffTimeRequest))
		offTime.DELETE(":id", wrap(srv.DeleteOffTimeRequest))
		offTime.POST(":id/approve", wrap(srv.ApproveOffTimeRequest))
		offTime.POST(":id/reject", wrap(srv.RejectOffTimeRequest))
	}

	constraints := v1.Group("constraint/")
	{
		constraints.GET("", wrap(srv.FindConstraints))
		constraints.POST("", wrap(srv.CreateConstraint))
		constraints.DELETE(":id", wrap(srv.DeleteConstraint))
	}

	worktime := v1.Group("worktime/")
	{
		worktime.POST("", wrap(srv.SetWorkTime))
		worktime.GET("", wrap(srv.GetCurrentWorkTimes))
		worktime.GET(":staff/history", wrap(srv.GetWorkTimeHistory))
	}

	return nil
}

func (srv *Server) listUsers(ctx context.Context) (map[string]structs.User, error) {
	token := middleware.JWTFromContext(ctx)
	res, err := srv.IdentityProvider.ListUsers(ctx, token)
	if err != nil {
		return nil, err
	}

	m := make(map[string]structs.User, len(res))
	for _, usr := range res {
		m[usr.Name] = usr
	}

	return m, nil
}

func wrap(fn HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		params := make(map[string]string)

		for _, name := range c.ParamNames() {
			params[name] = c.Param(name)
			middleware.AddLogFields(c, "param."+name, params[name])
		}

		res, err := fn(c.Request().Context(), c.QueryParams(), params, c.Request().Body)
		if err != nil {
			if merr, ok := err.(*multierror.Error); ok {
				errors := make([]string, len(merr.Errors))
				for idx, e := range merr.Errors {
					errors[idx] = e.Error()
				}

				return c.JSON(http.StatusBadRequest, map[string]any{
					"errors": errors,
				})
			}

			if errors.Is(err, mongo.ErrNoDocuments) {
				return c.JSON(http.StatusNotFound, map[string]any{
					"error": err.Error(),
				})
			}

			return err
		}

		if sw, ok := res.(*StatusWrapper); ok {
			if sw.Status == http.StatusNoContent {
				return c.NoContent(sw.Status)
			}

			return c.JSON(sw.Status, sw.Value)
		}

		return c.JSON(http.StatusOK, res)
	}
}

func (srv *Server) RequireAdmin(ctx context.Context) (any, bool) {
	// FIXME(ppacher)
	return nil, true
}
