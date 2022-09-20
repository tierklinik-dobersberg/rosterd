package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (srv *Server) ListWorkShifts(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	shifts, err := srv.Database.ListWorkShifts(ctx)

	if err != nil {
		return nil, err
	}

	return map[string]any{
		"workShifts": shifts,
	}, nil
}

func (srv *Server) CreateWorkShift(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	var shift structs.WorkShift

	decoder := json.NewDecoder(body)

	if err := decoder.Decode(&shift); err != nil {
		return nil, err
	}

	if err := validateNewWorkShift(shift); err != nil {
		return nil, err
	}

	if err := srv.Database.SaveWorkShift(ctx, &shift); err != nil {
		return nil, err
	}

	return withStatus(http.StatusCreated, shift)
}

func (srv *Server) UpdateWorkShift(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	var shift structs.WorkShift

	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&shift); err != nil {
		return nil, err
	}

	shiftID := params["id"]

	if err := validateNewWorkShift(shift); err != nil {
		return nil, err
	}

	var err error
	shift.ID, err = primitive.ObjectIDFromHex(shiftID)
	if err != nil {
		return nil, err
	}

	if err := srv.Database.SaveWorkShift(ctx, &shift); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) DeleteWorkShift(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	shiftID := params["id"]

	if err := srv.Database.DeleteWorkShift(ctx, shiftID); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}
