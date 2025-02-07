package roster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bufbuild/connect-go"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"github.com/tierklinik-dobersberg/rosterd/internal/timecalc"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (svc *RosterService) AnalyzeWorkTime(ctx context.Context, req *connect.Request[rosterv1.AnalyzeWorkTimeRequest]) (*connect.Response[rosterv1.AnalyzeWorkTimeResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing remote user"))
	}

	var (
		userIds []string
		err     error
	)
	if req.Msg.Users != nil {
		if req.Msg.Users.AllUsers {
			userIds, err = svc.FetchAllUserIds(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			userIds = req.Msg.Users.UserIds
		}
	} else {
		userIds = []string{remoteUser.ID}
	}

	res, err := svc.analyzeWorkTime(ctx, "", userIds, req.Msg.From, req.Msg.To, req.Msg.TimeTrackingOnly)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.AnalyzeWorkTimeResponse{
		Results: res,
	}), nil
}

func (svc *RosterService) analyzeWorkTime(ctx context.Context, rosterTypeName string, userIds []string, from, to string, onlyTimeTracking bool) ([]*rosterv1.WorkTimeAnalysis, error) {
	log.L(ctx).Infof("analyzing work time for users between %s and %s", from, to)

	// parse from and to times
	f, err := time.ParseInLocation("2006-01-02", from, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid from value %q: %w", from, err)
	}
	t, err := time.ParseInLocation("2006-01-02", to, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid to value %q: %w", to, err)
	}

	// fetch all distinct rosters
	distinctRosters := make(map[string]structs.DutyRoster)
	for iter := f; t.After(iter) || t.Equal(iter); iter = iter.AddDate(0, 0, 1) {
		rosters, err := svc.Datastore.DutyRostersByTime(ctx, iter)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch roster for %s: %w", iter, err)
		}

		for _, roster := range rosters {
			if rosterTypeName != "" && roster.RosterTypeName != rosterTypeName {
				slog.Info("skipping roster of different type", "requested", rosterTypeName, "roster-type", roster.RosterTypeName)
				continue
			}

			distinctRosters[roster.ID.Hex()] = roster
		}
	}

	log.L(ctx).Debugf("found %d distinct rosters that need to be analyzed", len(distinctRosters))

	// fetch all work shifts
	workShifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch work-shift definitions: %w", err)
	}

	// if there's a rosterTypeName only evaluate shifts that are allowed for the type.
	if rosterTypeName != "" {
		rosterType, err := svc.Datastore.GetRosterType(ctx, rosterTypeName)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch roster type name")
		}

		// iterate over the workshifts and only find shifts that are eligible for the
		// given roster type
		shifts := make([]structs.WorkShift, 0, len(workShifts))
		eligibleRoleIds := make(map[string]struct{})

		for _, shift := range workShifts {
			if data.ElemInBothSlices(shift.Tags, rosterType.ShiftTags) || data.ElemInBothSlices(shift.Tags, rosterType.OnCallTags) {
				shifts = append(shifts, shift)

				for _, role := range shift.EligibleRoles {
					eligibleRoleIds[role] = struct{}{}
				}
			} else {
				slog.Info("filtering workshift definitions since no tags overlap", "allowed-tags", rosterType.ShiftTags, "shift-tags", shift.Tags)
			}
		}
		roleIds := maps.Keys(eligibleRoleIds)

		workShifts = shifts

		// finally, fetch all user profiles and ensure we only calculate work times for eligible users
		profiles, err := svc.FetchAllUserProfiles(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user profiles: %w", err)
		}

		userMap := data.IndexSlice(profiles, func(p *idmv1.Profile) string {
			return p.User.Id
		})

		filteredIds := make([]string, 0, len(userIds))
		for _, id := range userIds {
			user, ok := userMap[id]
			if !ok {
				return nil, fmt.Errorf("failed to get user profile for id %q", id)
			}

			if user.User.Deleted {
				// skip deleted users here.
				continue
			}

			if data.ElemInBothSlicesFunc(roleIds, user.Roles, func(r *idmv1.Role) string {
				return r.Id
			}) {
				filteredIds = append(filteredIds, id)
			} else {
				slog.Info("filtering user since it's not eligble for the requested roster type", "user-id", id)
			}
		}
		userIds = filteredIds
	}

	// get the number of working-days
	holidays, err := svc.getHolidayLookupMap(ctx, f, t)
	if err != nil {
		return nil, err
	}

	monthlyWorkDays, err := timecalc.GatherWorkDaysByMonth(holidays, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to gather monthly work-days: %w", err)
	}

	// Get all worktimes
	perUserWorkTimes := make(map[string]timecalc.WorkTimeList, len(userIds))
	for _, id := range userIds {
		times, err := svc.Datastore.WorkTimeHistoryForStaff(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get work-time history for user %q: %w", id, err)
		}

		perUserWorkTimes[id] = times
	}

	expectedWorkTimes, err := timecalc.CalculateExpectedWorkTime(ctx, monthlyWorkDays, perUserWorkTimes, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate expected work time: %w", err)
	}

	plannedWorkTimes, err := timecalc.CalculatePlannedMonthlyWorkTime(ctx, maps.Values(distinctRosters), from, to, workShifts, perUserWorkTimes)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate planned work time: %w", err)
	}

	workTimeResult := make([]*rosterv1.WorkTimeAnalysis, 0, len(expectedWorkTimes))

	for userId := range expectedWorkTimes {
		var (
			expected     time.Duration
			plannedTotal time.Duration
		)

		planned := plannedWorkTimes.TotalForUser(userId)

		if onlyTimeTracking {
			expected = expectedWorkTimes[userId].TotalTrackedWorkTime()
			plannedTotal = planned.Tracked
		} else {
			expected = expectedWorkTimes[userId].TotalWorkTime()
			plannedTotal = planned.Total()
		}

		diff := plannedTotal - expected

		workTimeResult = append(workTimeResult, &rosterv1.WorkTimeAnalysis{
			UserId:       userId,
			PlannedTime:  durationpb.New(plannedTotal),
			ExpectedTime: durationpb.New(expected),
			Overtime:     durationpb.New(diff),
		})
	}

	return workTimeResult, nil
}
