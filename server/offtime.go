package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/middleware"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (srv *Server) CreateOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	var req structs.OffTimeRequest

	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&req); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	claims := middleware.ClaimsFromContext(ctx)

	_, isAdmin := srv.RequireAdmin(ctx)
	if !isAdmin || req.StaffID == "" {
		req.StaffID = claims.Subject
	}

	// Make sure the user is not able to overwrite an existing entry
	// by faking an existing object id
	req.ID = primitive.NilObjectID

	if err := validateNewOffTimeRequest(req); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"errors": unwrapErrors(err),
		})
	}

	if err := srv.Database.CreateOffTimeRequest(ctx, &req); err != nil {
		return nil, err
	}

	return req, nil
}

func (srv *Server) DeleteOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	req, err := srv.Database.GetOffTimeRequest(ctx, params["id"])
	if err != nil {
		return nil, err
	}

	_, isAdmin := srv.RequireAdmin(ctx)

	claims := middleware.ClaimsFromContext(ctx)
	if !isAdmin && req.StaffID != claims.Subject {
		return withStatus(http.StatusUnauthorized, map[string]any{
			"error": "operation not permitted",
		})
	}

	if req.Approved != nil {
		return withStatus(http.StatusPreconditionFailed, map[string]any{
			"error": "off-time request already approved/rejected, please contact an administrator",
		})
	}

	now := time.Now()
	if now.After(req.To) || now.After(req.From) {
		return withStatus(http.StatusPreconditionFailed, map[string]any{
			"error": "off-time request is in the past, please contact an administrator",
		})
	}

	if err := srv.Database.DeleteOffTimeRequest(ctx, req.ID.Hex()); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) FindOffTimeRequests(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {

	var (
		fromFilter     time.Time
		toFilter       time.Time
		staffFilter    []string
		approvedFilter *bool
	)

	if from := query.Get("from"); from != "" {
		var err error
		fromFilter, err = time.Parse("2006-01-02", from)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{
				"error": "invalid value for 'from' filter",
			})
		}
	}

	if to := query.Get("to"); to != "" {
		var err error
		toFilter, err = time.Parse("2006-01-02", to)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{
				"error": "invalid value for 'to' filter",
			})
		}
	}

	if approved := query.Get("approved"); approved != "" {
		b, err := strconv.ParseBool(approved)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{
				"error": "invalid value for 'approved' filter",
			})
		}

		approvedFilter = &b
	}

	_, isAdmin := srv.RequireAdmin(ctx)

	if isAdmin {
		claims := middleware.ClaimsFromContext(ctx)
		staffFilter = []string{
			claims.Subject,
		}
	} else {
		staffFilter = query["staff"]
	}

	req, err := srv.Database.FindOffTimeRequests(ctx, fromFilter, toFilter, approvedFilter, staffFilter)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"offTimeRequests": req,
	}, nil
}

func (srv *Server) ApproveOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	if err := srv.Database.ApproveOffTimeRequest(ctx, params["id"], true); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) RejectOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	if err := srv.Database.ApproveOffTimeRequest(ctx, params["id"], false); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}
