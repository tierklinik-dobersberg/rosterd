package roster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/mennanov/fmutils"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func (svc *RosterService) GetRequiredShifts(ctx context.Context, req *connect.Request[rosterv1.GetRequiredShiftsRequest]) (*connect.Response[rosterv1.GetRequiredShiftsResponse], error) {
	from, err := time.ParseInLocation("2006-01-02", req.Msg.From, time.Local)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid from value: %w", err))
	}

	to, err := time.ParseInLocation("2006-01-02", req.Msg.To, time.Local)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid to value: %w", err))
	}

	rosterType, err := svc.Datastore.GetRosterType(ctx, req.Msg.RosterTypeName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to get roster type %s", req.Msg.RosterTypeName))
		}

		return nil, err
	}

	tags := rosterType.ShiftTags
	if req.Msg.OnCall {
		tags = rosterType.OnCallTags

		if len(tags) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("roster type does not have on-call tags configured"))
		}
	}

	requiredShifts, definitions, _, workDays, err := svc.getRequiredShifts(ctx, from, to, nil, tags)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.GetRequiredShiftsResponse{
		RequiredShifts:       make([]*rosterv1.RequiredShift, len(requiredShifts)),
		WorkShiftDefinitions: make([]*rosterv1.WorkShift, len(definitions)),
		Days:                 workDays,
	}

	for idx, r := range requiredShifts {
		response.RequiredShifts[idx] = r.ToProto()
	}

	for idx, d := range definitions {
		response.WorkShiftDefinitions[idx] = d.ToProto()
	}

	if req.Msg.ReadMask != nil && len(req.Msg.ReadMask.Paths) > 0 {
		fmutils.Filter(response, req.Msg.ReadMask.Paths)
	}

	return connect.NewResponse(response), nil
}

func (svc *RosterService) GetUserShifts(ctx context.Context, req *connect.Request[rosterv1.GetUserShiftsRequest]) (*connect.Response[rosterv1.GetUserShiftsResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// validate the request
	if req.Msg.Timerange == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("missing timerange"))
	}

	if !req.Msg.Timerange.From.IsValid() || !req.Msg.Timerange.To.IsValid() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid from or to time"))
	}

	// get the time-range for which we want to load user-shifts.
	from := req.Msg.Timerange.From.AsTime().Local()
	to := req.Msg.Timerange.To.AsTime().Local()

	// gather all users where we want to return shifts.
	users := []string{remoteUser.ID}
	if req.Msg.Users != nil {
		if req.Msg.Users.AllUsers {
			log.L(ctx).Info("user requested working shifts for all users")

			var err error
			users, err = svc.FetchAllUserIds(ctx)

			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch users: %w", err))
			}
		} else if len(req.Msg.Users.UserIds) > 0 {
			users = req.Msg.Users.UserIds

			log.L(ctx).Info("user requested working shifts for specified users", "users", users)
		}
	}

	// collect all duty rosters between 'from' and 'to'
	rosters := make(map[string]structs.DutyRoster)
	for iter := from; iter.Before(to) || iter.Equal(to); iter = iter.AddDate(0, 0, 1) {
		rostersForDate, err := svc.Datastore.DutyRostersByTime(ctx, iter)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to search for duty rosters: %w", err))
		}

		for _, r := range rostersForDate {
			rosters[r.ID.Hex()] = r
		}
	}

	// load all workshift definitions
	workShiftDefinitions, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load workshift definitions: %w", err)
	}

	lm := data.IndexSlice(workShiftDefinitions, func(e structs.WorkShift) string { return e.ID.Hex() })

	var (
		plannedShifts    []structs.PlannedShift
		shiftDefinitions = make(map[string]structs.WorkShift)
	)

	for _, roster := range rosters {
		for _, shift := range roster.Shifts {

			// filter out shifts that are outside the requested time-range
			fromInRange := (from.Before(shift.From) || from.Equal(shift.From)) && (to.After(shift.From) || to.Equal(shift.From))
			toInRange := (from.Before(shift.To) || from.Equal(shift.To)) && (to.After(shift.To) || to.Equal(shift.To))
			if !fromInRange && !toInRange {
				continue
			}

			if data.ElemInBothSlices(shift.AssignedUserIds, users) {
				// filter out all ids that are not requested
				shift.AssignedUserIds = slices.DeleteFunc(shift.AssignedUserIds, func(id string) bool {
					return !slices.Contains(users, id)
				})

				plannedShifts = append(plannedShifts, shift)
				key := shift.WorkShiftID.Hex()
				shiftDefinitions[key] = lm[key]
			}
		}
	}

	res := &rosterv1.GetUserShiftsResponse{
		Shifts:      make([]*rosterv1.PlannedShift, 0, len(plannedShifts)),
		Definitions: make([]*rosterv1.WorkShift, 0, len(shiftDefinitions)),
	}

	for _, p := range plannedShifts {
		res.Shifts = append(res.Shifts, p.ToProto())
	}

	for _, def := range maps.Values(shiftDefinitions) {
		res.Definitions = append(res.Definitions, def.ToProto())
	}

	return connect.NewResponse(res), nil
}

func (svc *RosterService) getRequiredShifts(ctx context.Context, from, to time.Time, profiles *[]*idmv1.Profile, allowedTags []string) ([]structs.RequiredShift, []structs.WorkShift, []string, []*rosterv1.Day, error) {
	// fetch user profiles
	if profiles == nil {
		profiles = new([]*idmv1.Profile)
	}
	if len(*profiles) == 0 {
		var err error
		userProfiles, err := svc.FetchAllUserProfiles(ctx)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to fetch users: %w", err)
		}

		*profiles = make([]*idmv1.Profile, 0, len(userProfiles))

		for _, p := range userProfiles {
			if p.User.Deleted {
				continue
			}

			*profiles = append(*profiles, p)
		}
	}

	// fetch all holidays
	holidays, err := svc.getHolidayLookupMap(ctx, from, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	workDays := getWorkDays(ctx, holidays, from, to)

	// generate a list of required shifts
	var (
		results          = make([]structs.RequiredShift, 0)
		shiftLm          = make(map[string]struct{})
		shiftDefinitions = make([]structs.WorkShift, 0)
		eligibleUsers    = make(map[string]struct{})
	)

	for iter := from; to.After(iter) || to.Equal(iter); iter = iter.AddDate(0, 0, 1) {
		_, isHoliday := holidays[iter.Format("2006-01-02")]

		currentWorkTimes, err := svc.Datastore.GetCurrentWorkTimes(ctx, iter)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to get current work-times: %w", err)
		}

		shiftsPerDay, err := svc.Datastore.GetShiftsForDay(ctx, iter.Weekday(), isHoliday)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		for _, shift := range shiftsPerDay {
			// skip shifts that are marked as deleted.
			if shift.Deleted {
				continue
			}

			// filter shifts by tag
			if len(allowedTags) > 0 {
				allowed := false
				for _, tag := range shift.Tags {
					if slices.Contains(allowedTags, tag) {
						allowed = true
						break
					}
				}

				if !allowed {
					continue
				}
			}

			shiftStart, shiftEnd := shift.AtDay(iter)

			requiredShift := structs.RequiredShift{
				From:        shiftStart,
				To:          shiftEnd,
				WorkShiftID: shift.ID,
				OnHoliday:   isHoliday,
				OnWeekend:   iter.Weekday() == time.Saturday || iter.Weekday() == time.Sunday,
				Violations:  make(map[string]*rosterv1.ConstraintViolationList),
			}

			// add the shift definition if it's not already there.
			if _, ok := shiftLm[shift.ID.Hex()]; !ok {
				shiftDefinitions = append(shiftDefinitions, shift)
				shiftLm[shift.ID.Hex()] = struct{}{}
			}

			for _, profile := range *profiles {
				hasRole := data.ElemInBothSlicesFunc(shift.EligibleRoles, profile.Roles, func(role *idmv1.Role) string {
					return role.Id
				})

				if !hasRole {
					continue
				}

				eligibleUsers[profile.User.Id] = struct{}{}

				// check approved off-time requests
				approved := true
				offTimeRequests, err := svc.Datastore.FindOffTimeRequests(ctx, shiftStart, shiftEnd, &approved, []string{profile.User.Id})
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to load approved off-time requests for user %s: %w", profile.User.Id, err)
				}

				var (
					violations []*rosterv1.ConstraintViolation
					isEligible bool = true
				)

				// create a "fake" violation for each approved off-time-request
				for _, offReq := range offTimeRequests {
					isEligible = false
					violations = append(violations, &rosterv1.ConstraintViolation{
						Hard: true,
						Kind: &rosterv1.ConstraintViolation_OffTime{
							OffTime: &rosterv1.OffTimeViolation{
								Entry: offReq.ToProto(),
							},
						},
					})
				}

				wt, ok := currentWorkTimes[profile.User.Id]
				if !ok || (!wt.EndsWith.IsZero() && wt.EndsWith.Before(iter)) {
					isEligible = false
					violations = append(violations, &rosterv1.ConstraintViolation{
						Hard: true,
						Kind: &rosterv1.ConstraintViolation_NoWorkTime{
							NoWorkTime: true,
						},
					})
				}

				if ok && wt.ExcludeFromTimeTracking {
					// FIXME(ppacher): add a dedicated field for this
					violations = append(violations, &rosterv1.ConstraintViolation{
						Hard: false,
						Kind: &rosterv1.ConstraintViolation_Evaluation{
							Evaluation: &rosterv1.ConstraintEvaluationViolation{
								Description: "TimeTrackingDisabled",
							},
						},
					})
				}

				// check if the user is eligible or not
				if isEligible {
					requiredShift.EligibleUserIds = append(requiredShift.EligibleUserIds, profile.User.Id)
				}

				if len(violations) > 0 {
					if requiredShift.Violations[profile.User.Id] == nil {
						requiredShift.Violations[profile.User.Id] = &rosterv1.ConstraintViolationList{
							UserId: profile.User.Id,
						}
					}

					requiredShift.Violations[profile.User.Id].Violations = append(requiredShift.Violations[profile.User.Id].Violations, violations...)
				}
			}

			results = append(results, requiredShift)
		}
	}

	return results, shiftDefinitions, maps.Keys(eligibleUsers), workDays, nil
}
