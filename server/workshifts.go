package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

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

func (srv *Server) GetRequiredShifts(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	startTimeStr := query.Get("from")
	toTimeStr := query.Get("to")

	start, err := time.Parse("2006-01-02", startTimeStr)
	if err != nil {
		return nil, err
	}

	to, err := time.Parse("2006-01-02", toTimeStr)
	if err != nil {
		return nil, err
	}

	var shifts = make(map[string][]structs.RosterShift)

	for iter := start; to.After(iter); iter = iter.AddDate(0, 0, 1) {
		isHoliday, err := srv.Holidays.IsHoliday(ctx, srv.Country, iter)
		if err != nil {
			return nil, fmt.Errorf("failed loading holidays for %s: %w", iter, err)
		}

		shiftsPerDay, err := srv.Database.GetShiftsForDay(ctx, iter.Weekday(), isHoliday)
		if err != nil {
			return nil, fmt.Errorf("failed loading shiftss for %s: %w", iter, err)
		}

		rosterShifts := make([]structs.RosterShift, len(shiftsPerDay))
		for idx, shift := range shiftsPerDay {
			from, to := shift.AtDay(iter)

			worth := float64(to.Sub(from) / time.Minute)
			if shift.MinutesWorth != nil {
				worth = float64(*shift.MinutesWorth)
			}

			rosterShifts[idx] = structs.RosterShift{
				ShiftID:      shift.ID,
				Name:         shift.Name,
				IsHoliday:    isHoliday,
				IsWeekend:    from.Weekday() == time.Saturday || from.Weekday() == time.Sunday,
				From:         from,
				To:           to,
				MinutesWorth: worth,
			}
		}
		shifts[iter.Format("2006-01-02")] = rosterShifts
	}

	return shifts, nil
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
