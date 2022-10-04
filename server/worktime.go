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

func (srv *Server) NumberOfWorkingDays(ctx context.Context, from time.Time, to time.Time) (workDays int, weekEnd int, holiday int, err error) {
	for iter := from; iter.Before(to); iter = iter.AddDate(0, 0, 1) {
		switch iter.Weekday() {
		case time.Saturday, time.Sunday:
			weekEnd++
			continue
		default:
			isHoliday, err := srv.Holidays.IsHoliday(ctx, srv.Country, iter)
			if err != nil {
				return workDays, weekEnd, holiday, err
			}

			if isHoliday {
				holiday++
			} else {
				workDays++
			}
		}
	}

	return workDays, weekEnd, holiday, nil
}

func (srv *Server) calculateWorkTimeBetween(ctx context.Context, from, to time.Time) (map[string]time.Duration, map[string]structs.WorkTime, error) {
	numberOfWorkdays, _, _, err := srv.NumberOfWorkingDays(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	currentWorkTimes, err := srv.Database.GetCurrentWorkTimes(ctx, from)
	if err != nil {
		return nil, nil, err
	}

	result := make(map[string]time.Duration)

	for user, workTime := range currentWorkTimes {
		timePerWeekday := workTime.TimePerWeek / 5
		result[user] = timePerWeekday * time.Duration(numberOfWorkdays)
	}

	return result, currentWorkTimes, nil
}

func (srv *Server) calculateMonthlyWorkTime(ctx context.Context, month time.Month, year int) (map[string]time.Duration, error) {
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local)

	// find the number of working-days in the given month
	result, _, err := srv.calculateWorkTimeBetween(ctx, from, to)
	return result, err
}
