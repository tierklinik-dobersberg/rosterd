package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (srv *Server) CreateConstraint(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	var req structs.Constraint
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}

	if err := srv.Database.CreateConstraint(ctx, &req); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) DeleteConstraint(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	if err := srv.Database.DeleteConstraint(ctx, params["id"]); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) FindConstraints(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	res, err := srv.Database.FindConstraints(ctx, query["staff"], query["role"])
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"constraints": res,
	}, nil
}
