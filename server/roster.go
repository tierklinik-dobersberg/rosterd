package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ccssmnn/hego"
	"github.com/hashicorp/go-hclog"
	"github.com/tierklinik-dobersberg/rosterd/constraint"
	"github.com/tierklinik-dobersberg/rosterd/generator"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (srv *Server) CreateRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, isAdmin := srv.RequireAdmin(ctx); !isAdmin {
		return res, nil
	}

	var roster structs.Roster
	if err := json.NewDecoder(body).Decode(&roster); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	if err := srv.Database.CreateRoster(ctx, roster); err != nil {
		return nil, err
	}

	return nil, nil
}

func (srv *Server) UpdateRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, isAdmin := srv.RequireAdmin(ctx); !isAdmin {
		return res, nil
	}

	var roster structs.Roster
	if err := json.NewDecoder(body).Decode(&roster); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	var (
		rosterID = params["id"]
		err      error
	)
	roster.ID, err = primitive.ObjectIDFromHex(rosterID)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	if err := srv.Database.UpdateRoster(ctx, roster); err != nil {
		return nil, err
	}

	return nil, nil
}

func (srv *Server) DeleteRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, isAdmin := srv.RequireAdmin(ctx); !isAdmin {
		return res, nil
	}

	rosterID := params["id"]

	if err := srv.Database.DeleteRoster(ctx, rosterID); err != nil {
		return nil, err
	}

	return nil, nil
}

func (srv *Server) ApproveRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	if res, isAdmin := srv.RequireAdmin(ctx); !isAdmin {
		return res, nil
	}

	month, err := strconv.ParseInt(params["month"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	year, err := strconv.ParseInt(params["year"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	if err := srv.Database.ApproveRoster(ctx, time.Month(month), int(year)); err != nil {
		return nil, err
	}

	return nil, nil
}

func (srv *Server) FindRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	month, err := strconv.ParseInt(params["month"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	year, err := strconv.ParseInt(params["year"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	roster, err := srv.findRoster(ctx, time.Month(month), int(year))
	if err != nil {
		return nil, err
	}

	_, isAdmin := srv.RequireAdmin(ctx)
	if roster.Approved == nil && !isAdmin {
		return withStatus(http.StatusUnauthorized, nil)
	}

	return roster, nil
}

func (srv *Server) findRoster(ctx context.Context, month time.Month, year int) (*structs.Roster, error) {
	roster, err := srv.Database.FindRoster(ctx, month, year)
	if err != nil {
		return nil, err
	}

	// get a list of all workshift definitions.
	shifts, err := srv.Database.ListWorkShifts(ctx)
	if err != nil {
		return roster, err
	}

	// create a lookup map for shifts by their object ID.
	lookupMap := make(map[string]structs.WorkShift)
	for _, shift := range shifts {
		lookupMap[shift.ID.Hex()] = shift
	}

	// update all roster shifts and add a reference to the shift definition
	for idx, rosterShift := range roster.Shifts {
		roster.Shifts[idx].Definition = lookupMap[rosterShift.ShiftID.Hex()]
	}

	return roster, nil
}

func (srv *Server) FindCurrentlyWorkingStaff(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	date := time.Now()

	timeStr := query.Get("time")
	if timeStr != "" {
		var err error
		date, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
	}

	wholeDay := false

	dateStr := query.Get("date")
	if dateStr != "" {
		var err error
		date, err = time.ParseInLocation("2006-01-02", dateStr, srv.Location)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		wholeDay = true
	}

	roster, err := srv.findRoster(ctx, date.Month(), date.Year())
	if err != nil {
		return nil, err
	}

	// allow limiting the returned shifts by tags and shortName
	tags := query["tags"]
	shortNames := query["shortNames"]

	// get all shifts that are active right now
	var activeShifts []structs.RosterShift

	for _, shift := range roster.Shifts {
		if wholeDay {
			srv.Logger.Info("checking shift", "dateStr", dateStr, "shift", shift.From.UTC().Format("2006-01-02"))
		}

		if (wholeDay && shift.From.UTC().Format("2006-01-02") == dateStr) || ((shift.From.Before(date) || shift.From.Equal(date)) && (shift.To.After(date) || shift.To.Equal(date))) {
			if len(tags) > 0 || len(shortNames) > 0 {
				isAllowed := false

				for _, allowed := range shortNames {
					if shift.Definition.ShortName == allowed {
						isAllowed = true
						break
					}
				}

				// check tags
				if !isAllowed {
					for _, tag := range shift.Definition.Tags {
						for _, allowed := range tags {
							if tag == allowed {
								isAllowed = true
								break
							}
						}
					}
				}

				if !isAllowed {
					continue
				}
			}

			activeShifts = append(activeShifts, shift)
		}
	}

	// get a list of distinct users
	usersMap := map[string]struct{}{}
	users := []string{}
	for _, shift := range activeShifts {
		for _, staff := range shift.Staff {
			if _, ok := usersMap[staff]; ok {
				continue
			}

			usersMap[staff] = struct{}{}
			users = append(users, staff)
		}
	}

	return map[string]any{
		"staff":  users,
		"shifts": activeShifts,
	}, nil
}

func (srv *Server) GetRequiredShifts(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	startTimeStr := query.Get("from")
	toTimeStr := query.Get("to")
	includeStaffList := query.Has("stafflist")
	tags := query["tags"]

	start, err := time.ParseInLocation("2006-01-02", startTimeStr, srv.Location)
	if err != nil {
		return nil, err
	}

	to, err := time.ParseInLocation("2006-01-02", toTimeStr, srv.Location)
	if err != nil {
		return nil, err
	}

	shifts, _, err := srv.getRequiredShifts(ctx, start, to, includeStaffList, tags)

	return shifts, err
}

func (srv *Server) GenerateRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	year, err := strconv.ParseInt(params["year"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, nil)
	}

	month, err := strconv.ParseInt(params["month"], 0, 0)
	if err != nil {
		return withStatus(http.StatusBadRequest, nil)
	}

	// detect the from and to time for the roster
	start := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, srv.Location)
	to := time.Date(int(year), time.Month(month)+1, 0, 0, 0, 0, 0, srv.Location)

	requiredShifts, users, err := srv.getRequiredShifts(ctx, start, to, true, nil)
	if err != nil {
		return nil, err
	}
	userSlice := make([]structs.User, 0, len(users))
	for _, u := range users {
		userSlice = append(userSlice, u)
	}

	settings := hego.TSSettings{}
	settings.MaxIterations = 500
	settings.Verbose = settings.MaxIterations / 10
	settings.TabuListSize = 200
	settings.NeighborhoodSize = 20

	if val := query.Get("max-iterations"); val != "" {
		maxIter, err := strconv.ParseInt(val, 0, 0)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]string{
				"error": "invalid value for max-iterations",
			})
		}
		settings.MaxIterations = int(maxIter)
	}

	if val := query.Get("n-size"); val != "" {
		nSize, err := strconv.ParseInt(val, 0, 0)
		if err != nil {
			return withStatus(http.StatusBadRequest, map[string]string{
				"error": "invalid value for n-size",
			})
		}
		settings.NeighborhoodSize = int(nSize)
	}

	expectedWorkTime, err := srv.calculateMonthlyWorkTime(ctx, time.Month(month), int(year))
	if err != nil {
		return nil, err
	}

	cache := new(constraint.Cache)
	idx := 0
	generatorState := generator.NewGeneratorState(int(year), time.Month(month), requiredShifts, userSlice, expectedWorkTime, func(r structs.Roster) int {
		idx++
		res, err := srv.analyzeRoster(ctx, r, cache, true)
		if err != nil {
			srv.Logger.Error("failed to analyze roster", "error", err)
			return 1000000
		}

		srv.Logger.Info("evaluated generated roster", "run", idx, "objective", res.Penalty)

		return res.Penalty
	})

	initialState := generator.NewTabuState(*generatorState)

	res, err := hego.TS(initialState, settings)
	if err != nil {
		return withStatus(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})
	}

	state := res.BestState.(*generator.TabuState)

	r := state.ToRoster()

	analysisResult, err := srv.analyzeRoster(ctx, r, cache, true)
	if err != nil {
		hclog.L().Error("failed to do analysis for the generated roster", "error", err)
	}

	return map[string]any{
		"roster":   r,
		"analysis": analysisResult,
	}, nil
}

func (srv *Server) AnalyzeRoster(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	// Decode the roster that we should analyze
	var roster structs.Roster
	if err := json.NewDecoder(body).Decode(&roster); err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	return srv.analyzeRoster(ctx, roster, nil, true)
}

func (srv *Server) GetDayKinds(ctx context.Context, query url.Values, params map[string]string, body io.Reader) (any, error) {
	from, err := time.ParseInLocation("2006-01-02", params["from"], srv.Location)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	to, err := time.ParseInLocation("2006-01-02", params["to"], srv.Location)
	if err != nil {
		return withStatus(http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}

	workDays, weekEnds, holidays, err := srv.NumberOfWorkingDays(ctx, from, to)
	if err != nil {
		return withStatus(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})
	}

	return map[string]any{
		"workingDays":    workDays,
		"weekendDays":    weekEnds,
		"publicHolidays": holidays,
	}, nil
}

func (srv *Server) analyzeRoster(ctx context.Context, roster structs.Roster, cache *constraint.Cache, softConstraints bool) (*structs.RosterAnalysis, error) {
	if cache == nil {
		cache = new(constraint.Cache)
	}

	// detect the from and to time for the roster
	start := time.Date(roster.Year, roster.Month, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(roster.Year, roster.Month+1, 0, 0, 0, 0, 0, time.Local)

	// get a list of all required shifts but do not yet evaluate constraints
	requiredShifts, _, err := srv.getRequiredShifts(ctx, start, to, false, nil)
	if err != nil {
		return nil, err
	}

	users, err := srv.listUsers(ctx)
	if err != nil {
		return nil, err
	}

	var (
		diags            = []structs.Diagnostic{}
		workTimeOverview = map[string]*structs.WorkTimeStatus{}
	)

	// prepare the workTimeOverview for each user
	currentWorkTime, err := srv.Database.GetCurrentWorkTimes(ctx, start)
	if err != nil {
		return nil, err
	}

	// calculate the expected work time for this month based on the number of
	// actual working days (excluding weekend and public holidays)
	expectedWorkTime, err := srv.calculateMonthlyWorkTime(ctx, roster.Month, roster.Year)
	if err != nil {
		return nil, err
	}

	for name := range users {
		workTimeOverview[name] = &structs.WorkTimeStatus{
			TimePerWeek:           structs.JSDuration(currentWorkTime[name].TimePerWeek),
			OvertimePenaltyRatio:  currentWorkTime[name].OvertimePenaltyRatio,
			UndertimePenaltyRatio: currentWorkTime[name].UndertimePenaltyRatio,
			ExpectedWorkTime:      structs.JSDuration(expectedWorkTime[name]),
		}
	}

	// order the roster shifts in a map with the same key type as the requiredShifts
	// map
	rosterShifts := make(map[string][]structs.RosterShift)
	for _, shift := range roster.Shifts {
		key := shift.From.Format("2006-01-02")
		rosterShifts[key] = append(rosterShifts[key], shift)
	}

	// utility method to find a given roster shift
	getShift := func(key string, id primitive.ObjectID) *structs.RosterShift {
		for _, s := range rosterShifts[key] {
			if s.ShiftID.Hex() == id.Hex() {
				return &s
			}
		}

		return nil
	}

	// iterate all required shifts and make sure they fullfil the constraints
	// also, fill up the workTimes map with minutes of planned work-time per week and per user
	for key, shifts := range requiredShifts {
		for _, requiredShift := range shifts {
			// get the same shift from the roster
			rosterShift := getShift(key, requiredShift.ShiftID)
			if rosterShift == nil {
				diags = append(diags, structs.Diagnostic{
					Type:        "missing-shift",
					Description: "A shift is missing from the roster",
					Details:     requiredShift,
					Penalty:     constraint.MissingShiftPenalty,
					Date:        requiredShift.From.Format("2006-01-02"),
				})

				continue
			}

			shiftDiags, err := srv.validateRosterShift(ctx, softConstraints, users, roster, *rosterShift, requiredShift.RosterShift, cache)
			if err != nil {
				return nil, err
			}

			diags = append(diags, shiftDiags...)
			for _, staff := range rosterShift.Staff {
				workTimeOverview[staff].PlannedWorkTime += structs.JSDuration(rosterShift.MinutesWorth) * structs.JSDuration(time.Minute)
			}
		}
	}

	// for each user, evaluate roster-only constraints
	for user := range users {
		violations, err := cache.EvaluateForStaff(ctx, softConstraints, srv.Logger, srv.Database, user, users[user].Roles, structs.RosterShift{}, &roster, true)
		if err != nil {
			return nil, err
		}

		var sum int
		for _, violation := range violations {
			sum += violation.Panalty
		}

		if len(violations) > 0 {
			diags = append(diags, structs.Diagnostic{
				Type: "constraint-violation",
				Details: map[string]any{
					"user":       user,
					"violations": violations,
				},
				Penalty: sum,
			})
		}
	}

	var penaltySum int

	// finally, calculate the work-time difference between the expect amount and the actual planned
	// working time
	for user := range users {
		diff := float64(workTimeOverview[user].PlannedWorkTime-workTimeOverview[user].ExpectedWorkTime) / float64(time.Hour)

		var worktimePenalty int

		ratioOvertime := constraint.OverTimePenaltyFactor
		ratioUndertime := constraint.UnderTimePenaltyFactor

		if workTimeOverview[user].OvertimePenaltyRatio != 0 {
			ratioOvertime = workTimeOverview[user].OvertimePenaltyRatio
		}
		if workTimeOverview[user].UndertimePenaltyRatio != 0 {
			ratioUndertime = workTimeOverview[user].UndertimePenaltyRatio
		}

		switch {
		case diff < 0:
			worktimePenalty = int(-1.0 * diff * ratioUndertime)
		case diff > 0:
			worktimePenalty += int(diff * ratioOvertime)
		}

		workTimeOverview[user].Penalty = worktimePenalty
		penaltySum += worktimePenalty
	}

	for _, diag := range diags {
		penaltySum += diag.Penalty
	}

	return &structs.RosterAnalysis{
		Diagnostics: diags,
		WorkTime:    workTimeOverview,
		Penalty:     penaltySum,
	}, nil
}

func (srv *Server) validateRosterShift(ctx context.Context, softConstraints bool, users map[string]structs.User, roster structs.Roster, rosterShift structs.RosterShift, requiredShift structs.RosterShift, cache *constraint.Cache) ([]structs.Diagnostic, error) {
	var diags []structs.Diagnostic

	if len(rosterShift.Staff) < requiredShift.RequiredStaffCount {
		diags = append(diags, structs.Diagnostic{
			Type:        "missing-staff",
			Description: "There are not enough employees assigned for this shift",
			Penalty:     constraint.MissingStaffPenalty,
			Date:        requiredShift.From.Format("2006-01-02"),
			Details: map[string]any{
				"shiftID":       requiredShift.ShiftID,
				"shiftName":     requiredShift.Definition.Name,
				"requiredCount": requiredShift.RequiredStaffCount,
				"assignedStaff": rosterShift.Staff,
				"from":          requiredShift.From,
				"to":            requiredShift.To,
			},
		})
	}

	for _, user := range rosterShift.Staff {

		violations, err := cache.EvaluateForStaff(ctx, softConstraints, srv.Logger, srv.Database, user, users[user].Roles, rosterShift, &roster, false)
		if err != nil {
			return nil, err
		}

		// check off-time requests as well
		approved := true
		isCredit := false
		offTimeRequests, err := srv.Database.FindOffTimeRequests(ctx, rosterShift.From, rosterShift.To, &approved, []string{user}, &isCredit)
		if err != nil {
			return nil, err
		}

		for _, offReq := range offTimeRequests {
			violations = append(violations, structs.ConstraintViolation{
				ID:      offReq.ID,
				Name:    offReq.Description,
				Type:    "off-time",
				Panalty: constraint.OffTimePenalty,
			})
		}

		if len(violations) > 0 {
			var penaltySum int
			for _, v := range violations {
				penaltySum += v.Panalty
			}

			diags = append(diags, structs.Diagnostic{
				Type:        "constraint-violation",
				Description: "Constraint violations detected",
				Penalty:     penaltySum,
				Date:        requiredShift.From.Format("2006-01-02"),
				Details: map[string]any{
					"user":       user,
					"violations": violations,
				},
			})
		}
	}

	return diags, nil
}

func (srv *Server) getRequiredShifts(ctx context.Context, start, to time.Time, includeStaffList bool, tags []string) (map[string][]structs.RosterShiftWithStaffList, map[string]structs.User, error) {
	var shifts = make(map[string][]structs.RosterShiftWithStaffList)

	var allUsers map[string]structs.User
	if includeStaffList {
		var err error
		allUsers, err = srv.listUsers(ctx)
		if err != nil {
			return nil, nil, err
		}

		for user := range allUsers {
			if allUsers[user].Disabled != nil && *allUsers[user].Disabled {
				delete(allUsers, user)
			}
		}
	}

	// a small unitlity method to check a set of shift tags against
	// the set of filtered tags.
	// This returns true if no tag filter is set.
	hasFilteredTag := func(setTags []string) bool {
		if len(tags) == 0 {
			return true
		}

		for _, t := range setTags {
			for _, allowed := range tags {
				if allowed == t {
					return true
				}
			}
		}

		return false
	}

	for iter := start; to.After(iter); iter = iter.AddDate(0, 0, 1) {
		isHoliday, err := srv.Holidays.IsHoliday(ctx, srv.Country, iter)
		if err != nil {
			return nil, nil, fmt.Errorf("failed loading holidays for %s: %w", iter, err)
		}

		shiftsPerDay, err := srv.Database.GetShiftsForDay(ctx, iter.Weekday(), isHoliday)
		if err != nil {
			return nil, nil, fmt.Errorf("failed loading shiftss for %s: %w", iter, err)
		}

		rosterShifts := make([]structs.RosterShiftWithStaffList, len(shiftsPerDay))
		for idx, shift := range shiftsPerDay {
			if !hasFilteredTag(shift.Tags) {
				continue
			}

			from, to := shift.AtDay(iter)

			worth := float64(to.Sub(from) / time.Minute)
			if shift.MinutesWorth != nil && *shift.MinutesWorth > 0 {
				worth = float64(*shift.MinutesWorth)
			}
			rosterShift := structs.RosterShift{
				ShiftID:            shift.ID,
				IsHoliday:          isHoliday,
				IsWeekend:          from.Weekday() == time.Saturday || from.Weekday() == time.Sunday,
				From:               from,
				To:                 to,
				MinutesWorth:       worth,
				RequiredStaffCount: shift.RequiredStaffCount,
				Definition:         shift,
			}

			var eligibleStaff []string
			violationPerUser := make(map[string][]structs.ConstraintViolation)
			if includeStaffList {
				for _, u := range allUsers {
					violations, err := constraint.EvaluateForStaff(ctx, false, srv.Logger, srv.Database, u.Name, u.Roles, rosterShift, nil, false)
					if err != nil {
						return nil, nil, err
					}

					// check off-time requests as well
					approved := true
					isCredit := false
					offTimeRequests, err := srv.Database.FindOffTimeRequests(ctx, rosterShift.From, rosterShift.To, &approved, []string{u.Name}, &isCredit)
					if err != nil {
						return nil, nil, err
					}

					for _, offReq := range offTimeRequests {
						violations = append(violations, structs.ConstraintViolation{
							ID:   offReq.ID,
							Name: offReq.Description,
							Type: "off-time",
						})
					}

					if len(violations) == 0 {
						eligibleStaff = append(eligibleStaff, u.Name)
					} else {
						violationPerUser[u.Name] = violations
					}
				}
			}

			rosterShifts[idx] = structs.RosterShiftWithStaffList{
				RosterShift:   rosterShift,
				EligibleStaff: eligibleStaff,
				Violations:    violationPerUser,
			}
		}
		shifts[iter.Format("2006-01-02")] = rosterShifts
	}

	return shifts, allUsers, nil
}
