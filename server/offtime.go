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
)

func (srv *Server) CreateOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	var req structs.CreateOffTimeRequest

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

	actualWorkTime, workTimeStatus, err := srv.calculateWorkTimeBetween(ctx, req.From, req.To)
	if err != nil {
		return withStatus(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})
	}

	workTimeForStaff := actualWorkTime[req.StaffID]
	if workTimeForStaff == 0 {
		return withStatus(http.StatusPreconditionFailed, map[string]any{
			"error":   "No regular working time defined for staff",
			"staffID": req.StaffID,
		})
	}

	timePerDay := float64(workTimeStatus[req.StaffID].TimePerWeek / 5)

	costs := structs.OffTimeCosts{
		VacationDays: -1 * float64(actualWorkTime[req.StaffID]) / timePerDay,
		Duration:     -structs.JSDuration(actualWorkTime[req.StaffID]),
	}

	if req.RequestType == structs.RequestTypeCredits {
		return withStatus(http.StatusForbidden, map[string]any{
			"error": "wrong endpoint for giving vacation credits",
		})
	}

	entry := structs.OffTimeEntry{
		From:        req.From,
		To:          req.To,
		Description: req.Description,
		StaffID:     req.StaffID,
		RequestType: req.RequestType,
		CreatedAt:   time.Now(),
		CreatedBy:   claims.Subject,
		Costs:       costs,
	}

	if err := srv.Database.CreateOffTimeRequest(ctx, &entry); err != nil {
		return nil, err
	}

	return entry, nil
}

func (srv *Server) GetOffTimeCredits(ctx context.Context, _ url.Values, _ map[string]string, _ io.Reader) (any, error) {
	res, err := srv.Database.CalculateOffTimeCredits(ctx)
	if err != nil {
		return nil, err
	}

	_, isAdmin := srv.RequireAdmin(ctx)
	if !isAdmin {
		user := middleware.ClaimsFromContext(ctx)
		return map[string]any{
			user.Subject: res[user.Subject],
		}, nil
	}

	return res, nil
}

func (srv *Server) AddOffTimeCredit(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {

	claims := middleware.ClaimsFromContext(ctx)
	if res, isAdmin := srv.RequireAdmin(ctx); !isAdmin {
		return res, nil
	}

	var req structs.CreateOffTimeCreditsRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	if req.From.IsZero() {
		req.From = time.Now()
	}

	currentWorkTimes, err := srv.Database.GetCurrentWorkTimes(ctx, req.From)
	if err != nil {
		return withStatus(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})
	}

	timePerDay := float64(currentWorkTimes[params["staff"]].TimePerWeek) / 5

	entry := structs.OffTimeEntry{
		From:        req.From,
		Description: req.Description,
		StaffID:     req.StaffID,
		RequestType: structs.RequestTypeCredits,
		CreatedAt:   time.Now(),
		CreatedBy:   claims.Subject,
		Approval: &structs.Approval{
			Approved:   true,
			ApprovedAt: time.Now(),
			ActualCosts: structs.OffTimeCosts{
				VacationDays: req.Days,
				Duration:     structs.JSDuration(timePerDay * req.Days),
			},
		},
	}

	if err := srv.Database.CreateOffTimeRequest(ctx, &entry); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
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

	if req.Approval != nil {
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
		fromFilter, err = time.ParseInLocation("2006-01-02", from, srv.Location)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{
				"error": "invalid value for 'from' filter",
			})
		}
	}

	if to := query.Get("to"); to != "" {
		var err error
		toFilter, err = time.ParseInLocation("2006-01-02", to, srv.Location)
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

	req, err := srv.Database.FindOffTimeRequests(ctx, fromFilter, toFilter, approvedFilter, staffFilter, nil)
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

	var payload structs.ApproveOffTimeRequestRequest
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	req, err := srv.Database.GetOffTimeRequest(ctx, params["id"])
	if err != nil {
		return withStatus(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})
	}

	costs := req.Costs
	if payload.DurationCosts != nil {
		workTimePerWeek, err := srv.Database.GetCurrentWorkTimes(ctx, req.From)
		if err != nil {
			return withStatus(http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
		}

		timePerDay := float64(workTimePerWeek[req.StaffID].TimePerWeek) / 5
		costs.Duration = *payload.DurationCosts
		costs.VacationDays = float64(*payload.DurationCosts) / timePerDay
	}

	approval := structs.Approval{
		Approved:    true,
		Comment:     payload.Comment,
		ApprovedAt:  time.Now(),
		ActualCosts: costs,
	}

	if err := srv.Database.ApproveOffTimeRequest(ctx, params["id"], &approval); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}

func (srv *Server) RejectOffTimeRequest(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, ok := srv.RequireAdmin(ctx); !ok {
		return res, nil
	}

	var payload structs.RejectOffTimeRequestRequest
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	approval := &structs.Approval{
		Approved:   false,
		ApprovedAt: time.Now(),
		Comment:    payload.Comment,
		ActualCosts: structs.OffTimeCosts{
			VacationDays: 0,
			Duration:     0,
		},
	}

	if err := srv.Database.ApproveOffTimeRequest(ctx, params["id"], approval); err != nil {
		return nil, err
	}

	return withStatus(http.StatusNoContent, nil)
}
