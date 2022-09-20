package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (srv *Server) SetWorkTime(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	var wt structs.WorkTime
	if err := json.NewDecoder(body).Decode(&wt); err != nil {
		return nil, err
	}

	if wt.ApplicableFrom.IsZero() {
		wt.ApplicableFrom = time.Now()
	}

	if wt.Staff == "" {
		return withStatus(http.StatusBadRequest, "missing staff name")
	}

	if wt.TimePerWeek == 0 {
		return withStatus(http.StatusBadRequest, "missing timePerWeek")
	}

	if err := srv.Database.SaveWorkTimePerWeek(ctx, &wt); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) GetWorkTimeHistory(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	staff := params["staff"]

	res, err := srv.Database.WorkTimeHistoryForStaff(ctx, staff)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"history": res,
	}, nil
}

func (srv *Server) GetCurrentWorkTimes(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	res, err := srv.Database.GetCurrentWorkTimes(ctx, time.Now())
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"workTimes": res,
	}, nil
}
