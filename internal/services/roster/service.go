package roster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/mennanov/fmutils"
	"github.com/sirupsen/logrus"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/config"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/slices"
)

type RosterService struct {
	rosterv1connect.UnimplementedRosterServiceHandler

	*config.Providers
}

func NewRosterService(p *config.Providers) *RosterService {
	return &RosterService{
		Providers: p,
	}
}

func (svc *RosterService) ReapplyShiftTimes(ctx context.Context, req *connect.Request[rosterv1.ReapplyShiftTimesRequest]) (*connect.Response[rosterv1.ReapplyShiftTimesResponse], error) {
	roster, err := svc.Datastore.DutyRosterByID(ctx, req.Msg.RosterId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, err
	}

	// load all workshift definitions
	shifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load work shifts: %w", err)
	}

	shiftMap := data.IndexSlice(shifts, func(shift structs.WorkShift) string { return shift.ID.Hex() })

	// re apply shift times
	for idx, shift := range roster.Shifts {
		def, ok := shiftMap[shift.WorkShiftID.Hex()]
		if !ok {
			return nil, fmt.Errorf("failed to get workshift definition for planned shift ID %q", shift.WorkShiftID.Hex())
		}

		start, end := def.AtDay(shift.From.Local())

		timeWorth := shift.To.Sub(shift.From)
		if def.MinutesWorth != nil {
			timeWorth = time.Duration(*def.MinutesWorth) * time.Minute
		}

		if !shift.From.Equal(start) || !shift.To.Equal(end) || shift.TimeWorth != timeWorth {
			slog.Info("updating shift times", "name", def.Name, "oldStart", shift.From.Local(), "oldEnd", shift.To.Local(), "newStart", start, "newEnd", end, "oldTimeWorth", shift.TimeWorth, "newTimeWorth", timeWorth)

			shift.From = start
			shift.To = end
			shift.TimeWorth = timeWorth

			roster.Shifts[idx] = shift
		}
	}

	if _, err := svc.Datastore.SaveDutyRoster(ctx, &roster, nil); err != nil {
		return nil, fmt.Errorf("failed to save roster: %w", err)
	}

	svc.Providers.PublishEvent(&rosterv1.RosterChangedEvent{
		Roster: roster.ToProto(),
	}, false)

	return connect.NewResponse(&rosterv1.ReapplyShiftTimesResponse{
		Roster: roster.ToProto(),
	}), nil
}

func (svc *RosterService) SaveRoster(ctx context.Context, req *connect.Request[rosterv1.SaveRosterRequest]) (*connect.Response[rosterv1.SaveRosterResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	var (
		roster   structs.DutyRoster
		casIndex *uint64
	)

	if req.Msg.Id == "" {
		roster = structs.DutyRoster{
			From:           req.Msg.From,
			To:             req.Msg.To,
			CreatedAt:      time.Now(),
			LastModifiedBy: remoteUser.ID,
			UpdatedAt:      time.Now(),
			ShiftTags:      req.Msg.ShiftTags,
			RosterTypeName: req.Msg.RosterTypeName,
			CASIndex:       0,
		}

		casIndex = nil
	} else {
		var err error

		roster, err = svc.Datastore.DutyRosterByID(ctx, req.Msg.Id)
		if err != nil {
			return nil, err
		}

		if roster.CASIndex != req.Msg.CasIndex {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("CAS index conflict"))
		}

		casIndex = &req.Msg.CasIndex
	}

	rosterType, err := svc.Datastore.GetRosterType(ctx, roster.RosterTypeName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find roster type with name %q", roster.RosterTypeName))
		}

		return nil, err
	}

	// load all required shift definitions for this roster
	_, definitions, users, _, err := svc.getRequiredShifts(ctx, roster.FromTime(), roster.ToTime(), nil, rosterType.ShiftTags)
	if err != nil {
		return nil, fmt.Errorf("failed to get required shifts for roster: %w", err)
	}

	workShiftLm := data.IndexSlice(definitions, func(e structs.WorkShift) string { return e.ID.Hex() })

	roster.Shifts = make([]structs.PlannedShift, len(req.Msg.Shifts))
	for idx, shift := range req.Msg.Shifts {
		var conv structs.PlannedShift

		if err := conv.FromProto(shift); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid shift definition: %w", err))
		}

		def, ok := workShiftLm[shift.WorkShiftId]
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("work shift with id %q is not allowed for roster type %q", shift.WorkShiftId, rosterType.UniqueName))
		}

		// update the time worth field
		conv.TimeWorth = conv.To.Sub(conv.From)
		if def.MinutesWorth != nil {
			conv.TimeWorth = time.Duration(*def.MinutesWorth) * time.Minute
		}

		// ensure from and to times are valid
		shiftFrom, shiftTo := def.AtDay(conv.From)
		if !shiftFrom.Equal(conv.From) || !shiftTo.Equal(conv.To) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("shift from and to times do not match"))
		}

		roster.Shifts[idx] = conv
	}

	if roster.IsApproved() && !req.Msg.KeepApproval {
		// reset approval fields
		roster.Approved = false
		roster.ApprovedAt = time.Time{}
		roster.ApproverUserId = ""

		oldRosterID := roster.ID

		// generate a new id for the roster and reset the CAS index values
		roster.ID = primitive.NewObjectID()
		roster.CASIndex = 0
		casIndex = nil

		log.L(ctx).Infof("marking approved roster %q as superseded by %s", oldRosterID.Hex(), roster.ID.Hex())

		// remove the approval since this roster has been modified
		if err := svc.Datastore.DeleteOffTimeCostsByRoster(ctx, oldRosterID.Hex()); err != nil {
			return nil, fmt.Errorf("failed to delete off-time costs for an already approved roster: %w", err)
		}

		// mark the old duty roster as deleted and superseded by the new roster ID
		if err := svc.Datastore.DeleteDutyRoster(ctx, oldRosterID.Hex(), roster.ID); err != nil {
			return nil, fmt.Errorf("failed to mark updated duty roster with id %q as superseded (deleted): %s", oldRosterID.Hex(), err)
		}
	}

	if _, err := svc.Datastore.SaveDutyRoster(ctx, &roster, casIndex); err != nil {
		return nil, err
	}

	// caculate the work-time for the roster
	analysis, err := svc.analyzeWorkTime(ctx, roster.RosterTypeName, users, roster.From, roster.To, req.Msg.TimeTrackingOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate work-time: %w", err)
	}

	svc.Providers.PublishEvent(&rosterv1.RosterChangedEvent{
		Roster: roster.ToProto(),
	}, false)

	response := &rosterv1.SaveRosterResponse{
		Roster:           roster.ToProto(),
		WorkTimeAnalysis: analysis,
	}

	if req.Msg.ReadMask != nil && len(req.Msg.ReadMask.Paths) > 0 {
		fmutils.Filter(response, req.Msg.ReadMask.Paths)
	}

	return connect.NewResponse(response), nil
}

func (svc *RosterService) DeleteRoster(ctx context.Context, req *connect.Request[rosterv1.DeleteRosterRequest]) (*connect.Response[rosterv1.DeleteRosterResponse], error) {
	if err := svc.Datastore.DeleteOffTimeCostsByRoster(ctx, req.Msg.Id); err != nil {
		return nil, fmt.Errorf("failed to delete off-time costs for roster id %s. Please contact your administrator: %w", req.Msg.Id, err)
	}

	if err := svc.Datastore.DeleteDutyRoster(ctx, req.Msg.Id, primitive.NilObjectID); err != nil {
		return nil, fmt.Errorf("failed to delete roster with id %s. Please contact your administrator: %w", req.Msg.Id, err)
	}

	return connect.NewResponse(&rosterv1.DeleteRosterResponse{}), nil
}

func (svc *RosterService) ApproveRoster(ctx context.Context, req *connect.Request[rosterv1.ApproveRosterRequest]) (*connect.Response[rosterv1.ApproveRosterResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// first, fetch the roster from the database
	roster, err := svc.Datastore.DutyRosterByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	allUserIds, err := svc.FetchAllUserIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ids: %w", err)
	}

	fromTime := roster.FromTime()

	// caculate the work-time for the roster. The last parameter specified
	// that we only want work-time analysis for users with time-tracking enabled.
	analysis, err := svc.analyzeWorkTime(ctx, roster.RosterTypeName, allUserIds, roster.From, roster.To, true)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate work-time: %w", err)
	}

	approver := remoteUser.ID

	// Validate off-time costs split first
	for _, an := range analysis {
		diff := an.Overtime.AsDuration()

		if diff < 0 {
			var split *rosterv1.ApproveRosterWorkTimeSplit
			for _, splits := range req.Msg.WorkTimeSplit {
				if splits.UserId == an.UserId {
					split = splits
					break
				}
			}

			timeOffCosts := diff
			var vacationCosts time.Duration

			// administrator did not specify how to handle the undertime,
			// we fall back as normal offtime-costs
			if split != nil {
				timeOffCosts = split.TimeOff.AsDuration()
				vacationCosts = split.Vacation.AsDuration()
			}

			if timeOffCosts > 0 || vacationCosts > 0 {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid off-time cost split"))
			}

			if (timeOffCosts + vacationCosts) != diff {
				return nil, fmt.Errorf("invalid off-time cost split, time-off=%q, vacation=%q, sum must equal %q", timeOffCosts, vacationCosts, diff)
			}
		}
	}

	// If the roster was already approved and we "re-approve" it, make sure to recalculate the off-time
	// costs.
	if roster.IsApproved() {
		if err := svc.Datastore.DeleteOffTimeCostsByRoster(ctx, roster.ID.Hex()); err != nil {
			return nil, fmt.Errorf("failed to remove off-time costs bound to the roster: %w", err)
		}
	}

	for _, an := range analysis {
		if an.ExcludeFromTimeTracking {
			continue
		}

		diff := an.Overtime.AsDuration()

		if diff > 0 {
			// user has more time planned than was expected, add this as some
			// off-time credits.
			log.L(ctx).
				WithFields(logrus.Fields{
					"user":     an.UserId,
					"overtime": diff.String(),
				}).
				Infof("adding off-time costs entry for over-time")

			if err := svc.Datastore.AddOffTimeCost(ctx, &structs.OffTimeCosts{
				UserID:    an.UserId,
				RosterID:  roster.ID,
				CreatorId: approver,
				CreatedAt: time.Now(),
				Costs:     diff,
				Date:      fromTime,
			}); err != nil {
				return nil, fmt.Errorf("failed to add off-time credits for user %s: %w", an.UserId, err)
			}
		} else if diff < 0 {
			var split *rosterv1.ApproveRosterWorkTimeSplit
			for _, splits := range req.Msg.WorkTimeSplit {
				if splits.UserId == an.UserId {
					split = splits
					break
				}
			}

			// if administrator did not specify how to handle the undertime,
			// we fall back as normal offtime-costs
			timeOffCosts := diff
			var vacationCosts time.Duration

			// use the split if specified
			if split != nil {
				timeOffCosts = split.TimeOff.AsDuration()
				vacationCosts = split.Vacation.AsDuration()
			}

			if timeOffCosts < 0 {
				log.L(ctx).
					WithFields(logrus.Fields{
						"user":      an.UserId,
						"undertime": timeOffCosts.String(),
					}).
					Infof("adding off-time costs entry for undertime")

				if err := svc.Datastore.AddOffTimeCost(ctx, &structs.OffTimeCosts{
					UserID:    an.UserId,
					RosterID:  roster.ID,
					CreatorId: approver,
					CreatedAt: time.Now(),
					Costs:     timeOffCosts,
					Date:      fromTime,
				}); err != nil {
					return nil, fmt.Errorf("failed to add off-time credits for user %s: %w", an.UserId, err)
				}
			}

			if vacationCosts < 0 {
				log.L(ctx).
					WithFields(logrus.Fields{
						"user":     an.UserId,
						"vacation": vacationCosts.String(),
					}).
					Infof("adding off-time costs entry for vacation")

				if err := svc.Datastore.AddOffTimeCost(ctx, &structs.OffTimeCosts{
					UserID:     an.UserId,
					RosterID:   roster.ID,
					CreatorId:  approver,
					CreatedAt:  time.Now(),
					Costs:      vacationCosts,
					Date:       fromTime,
					IsVacation: true,
				}); err != nil {
					return nil, fmt.Errorf("failed to add off-time credits for user %s: %w", an.UserId, err)
				}
			}
		}
	}

	if err := svc.Datastore.ApproveDutyRoster(ctx, req.Msg.Id, approver); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.ApproveRosterResponse{}), nil
}

func (svc *RosterService) GetWorkingStaff(ctx context.Context, req *connect.Request[rosterv1.GetWorkingStaffRequest]) (*connect.Response[rosterv1.GetWorkingStaffResponse], error) {
	oldReq := &rosterv1.GetWorkingStaffRequest2{
		Query: &rosterv1.GetWorkingStaffRequest2_Time{
			Time: req.Msg.Time,
		},
		ReadMaks:       req.Msg.ReadMaks,
		RosterTypeName: req.Msg.RosterTypeName,
		OnCall:         req.Msg.OnCall,
	}

	return svc.GetWorkingStaff2(ctx, connect.NewRequest(oldReq))
}

func (svc *RosterService) GetWorkingStaff2(ctx context.Context, req *connect.Request[rosterv1.GetWorkingStaffRequest2]) (*connect.Response[rosterv1.GetWorkingStaffResponse], error) {
	if req.Msg.OnCall {
		if req.Msg.RosterTypeName == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("on_call may only be set if roster_type_name is set"))
		}
	}

	var (
		rosters []structs.DutyRoster
		err     error

		checkShift func(shift structs.PlannedShift) bool
	)

	switch v := req.Msg.GetQuery().(type) {
	case *rosterv1.GetWorkingStaffRequest2_Time:
		t := time.Now()

		if v.Time.IsValid() {
			t = v.Time.AsTime()
		}

		rosters, err = svc.Datastore.FindRostersWithActiveShifts(ctx, t)
		checkShift = func(shift structs.PlannedShift) bool {
			return (shift.From.Before(t) || shift.From.Equal(t)) && (shift.To.After(t) || shift.To.Equal(t))
		}

	case *rosterv1.GetWorkingStaffRequest2_TimeRange:
		var (
			from, to time.Time
		)

		if v.TimeRange.From.IsValid() {
			from = v.TimeRange.From.AsTime()
		}

		if v.TimeRange.To.IsValid() {
			to = v.TimeRange.To.AsTime()
		}

		rosters, err = svc.Datastore.FindRostersWithActiveShiftsInRange(ctx, from, to)
		checkShift = func(shift structs.PlannedShift) bool {
			if !from.IsZero() {
				if shift.To.Before(from) {
					return false
				}
			}

			if !to.IsZero() {
				if shift.From.After(to) {
					return false
				}
			}

			return true
		}
	}

	if err != nil {
		return nil, err
	}

	if len(rosters) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find any rosters"))
	}

	shifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch work shift definitions: %w", err)
	}

	shiftMap := make(map[string]structs.WorkShift)
	for _, s := range shifts {
		shiftMap[s.ID.Hex()] = s
	}

	relevantShifts := make([]structs.PlannedShift, 0)
	relevantRosters := make(map[string]struct{})

	// Load the roster type from the datastore if the name is set in the
	// request.
	var rosterType structs.RosterType
	if req.Msg.RosterTypeName != "" {
		rosterType, err = svc.Datastore.GetRosterType(ctx, req.Msg.RosterTypeName)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to get roster type with name %q", req.Msg.RosterTypeName))
			}

			return nil, err
		}
	}

	orderedUserIds := make([]string, 0)
	userIds := make(map[string]struct{})
	for _, roster := range rosters {
		if rosterType.UniqueName != "" && roster.RosterTypeName != rosterType.UniqueName {
			continue
		}

		log.L(ctx).Debugf("checking roster %s (from %s to %s) with type %s", roster.ID.Hex(), roster.From, roster.To, roster.RosterTypeName)

		// Sort roster shifts first
		sort.Stable(&rosterShiftSlice{
			shifts: roster.Shifts,
			defs:   shiftMap,
		})

		for _, shift := range roster.Shifts {
			def, ok := shiftMap[shift.WorkShiftID.Hex()]
			if !ok {
				return nil, fmt.Errorf("failed to get shift definition for id %s", shift.WorkShiftID.Hex())
			}

			// check if the shift is filtered by tag.
			isAllowed := len(rosterType.ShiftTags) == 0 && len(rosterType.OnCallTags) == 0

			if !isAllowed {
				// Check if we should only return staff that is assigned to on-call
				// shifts.
				if req.Msg.OnCall {
					isAllowed = data.ElemInBothSlices(rosterType.OnCallTags, def.Tags)
				} else {
					isAllowed = data.ElemInBothSlices(rosterType.ShiftTags, def.Tags)
				}
			}

			if len(req.Msg.ShiftTags) > 0 {
				for _, tag := range req.Msg.ShiftTags {
					if !slices.Contains(def.Tags, tag) {
						isAllowed = false
						break
					}
				}
			}

			if !isAllowed {
				log.L(ctx).Debugf("shift %s is filtered. shift-tags=%v rosterType.shiftTags=%v rosterType.onCallTags=%v", shift.WorkShiftID, def.Tags, rosterType.ShiftTags, rosterType.OnCallTags)

				continue
			}

			if checkShift(shift) {
				relevantShifts = append(relevantShifts, shift)
				relevantRosters[roster.ID.Hex()] = struct{}{}
				for _, user := range shift.AssignedUserIds {
					if _, ok := userIds[user]; !ok {
						userIds[user] = struct{}{}
						orderedUserIds = append(orderedUserIds, user)
					}
				}
			} else {
				log.L(ctx).Debugf("shift is either before or after the requested time")
			}
		}
	}

	response := &rosterv1.GetWorkingStaffResponse{
		UserIds: orderedUserIds,
	}

	for _, shift := range relevantShifts {
		response.CurrentShifts = append(response.CurrentShifts, shift.ToProto())
	}
	for roster := range relevantRosters {
		response.RosterId = append(response.RosterId, roster)
	}

	return connect.NewResponse(response), nil
}

func (svc *RosterService) GetRoster(ctx context.Context, req *connect.Request[rosterv1.GetRosterRequest]) (*connect.Response[rosterv1.GetRosterResponse], error) {
	var (
		dutyRoster         []structs.DutyRoster
		canIncludeWorktime bool
	)

	switch v := req.Msg.Search.(type) {
	case *rosterv1.GetRosterRequest_Date:
		if !v.Date.IsValid() {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid date"))
		}

		var err error
		dutyRoster, err = svc.Datastore.DutyRostersByTime(ctx, v.Date.AsTime())
		if err != nil {
			return nil, err
		}

	case *rosterv1.GetRosterRequest_DateString:
		t, err := time.ParseInLocation("2006-01-02", v.DateString, time.Local)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid value for field 'date_string': %w", err))
		}

		dutyRoster, err = svc.Datastore.DutyRostersByTime(ctx, t)
		if err != nil {
			return nil, err
		}

	case *rosterv1.GetRosterRequest_Id:
		r, err := svc.Datastore.DutyRosterByID(ctx, v.Id)
		if err != nil {
			return nil, err
		}

		dutyRoster = []structs.DutyRoster{r}

		canIncludeWorktime = true

	default:
		var err error

		dutyRoster, err = svc.Datastore.LoadDutyRosters(ctx)
		if err != nil {
			return nil, err
		}
	}

	// filter rosters by type name.
	if len(req.Msg.RosterTypeNames) > 0 {
		listCopy := make([]structs.DutyRoster, 0, len(dutyRoster))
		for _, roster := range dutyRoster {
			if slices.Contains(req.Msg.RosterTypeNames, roster.RosterTypeName) {
				listCopy = append(listCopy, roster)
			}
		}

		dutyRoster = listCopy
	}

	// check if we should include the work-time analysis as well.
	shouldIncludeAnalysis := false
	if req.Msg.ReadMask == nil || len(req.Msg.ReadMask.Paths) == 0 {
		shouldIncludeAnalysis = true
	}
	if !shouldIncludeAnalysis && req.Msg.ReadMask != nil {
		for _, path := range req.Msg.ReadMask.Paths {
			if strings.HasPrefix(path, "work_time_analysis") {
				shouldIncludeAnalysis = true
				break
			}
		}
	}

	response := &rosterv1.GetRosterResponse{
		Roster: make([]*rosterv1.Roster, len(dutyRoster)),
	}

	for idx, r := range dutyRoster {
		response.Roster[idx] = r.ToProto()
	}

	if canIncludeWorktime && shouldIncludeAnalysis && len(dutyRoster) == 1 {
		allUserIds, err := svc.FetchAllUserIds(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get user ids: %w", err)
		}

		// caculate the work-time for the roster
		analysis, err := svc.analyzeWorkTime(ctx, dutyRoster[0].RosterTypeName, allUserIds, dutyRoster[0].From, dutyRoster[0].To, req.Msg.TimeTrackingOnly)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate work-time: %w", err)
		}

		response.WorkTimeAnalysis = analysis
	} else if shouldIncludeAnalysis && !canIncludeWorktime && len(dutyRoster) > 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("time tracking analysis is not allowed"))
	}

	if req.Msg.ReadMask != nil && len(req.Msg.ReadMask.Paths) > 0 {
		fmutils.Filter(response, req.Msg.ReadMask.Paths)
	}

	return connect.NewResponse(response), nil
}

func (svc *RosterService) getHolidayLookupMap(ctx context.Context, from time.Time, to time.Time) (map[string]*calendarv1.PublicHoliday, error) {
	holidaysToFetch := []time.Time{from}
	if from.Year() != to.Year() || from.Month() != to.Month() {
		holidaysToFetch = append(holidaysToFetch, to)
	}

	var holidays []*calendarv1.PublicHoliday
	for _, t := range holidaysToFetch {
		res, err := svc.Holidays.GetHoliday(ctx, connect.NewRequest(&calendarv1.GetHolidayRequest{
			Year:  uint64(t.Year()),
			Month: uint64(t.Month()),
		}))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch holidays for %s", t.Format("2006-01-02"))
		}

		holidays = append(holidays, res.Msg.Holidays...)
	}
	holidayLookupMap := make(map[string]*calendarv1.PublicHoliday, len(holidays))
	for _, holiday := range holidays {
		holidayLookupMap[holiday.Date] = holiday
	}

	return holidayLookupMap, nil
}

func getWorkDays(_ context.Context, holidays map[string]*calendarv1.PublicHoliday, from time.Time, to time.Time) []*rosterv1.Day {
	var dayTypes []*rosterv1.Day

	until := to.Format("2006-01-02")
	for iter := from; iter.Format("2006-01-02") != until; iter = iter.AddDate(0, 0, 1) {
		key := iter.Format("2006-01-02")

		if hd, ok := holidays[key]; ok && hd.Type == calendarv1.HolidayType_PUBLIC {
			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_HOLIDAY,
			})

			continue
		}

		switch iter.Weekday() {
		case time.Saturday, time.Sunday:
			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_WEEKEND,
			})
		default:
			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_WORKDAY,
			})
		}

	}

	return dayTypes
}

func daysInMonth(ctx context.Context, holidays map[string]*calendarv1.PublicHoliday, m time.Month, year int, notBefore, notAfter time.Time, includeNotAfter bool) (int, []*rosterv1.Day) {
	firstDayInMonth := time.Date(year, m, 1, 0, 0, 0, 0, time.Local)

	var (
		numberOfWorkingDays int
		dayTypes            []*rosterv1.Day
	)

	for iter := firstDayInMonth; iter.Month() == m; iter = iter.AddDate(0, 0, 1) {
		key := iter.Format("2006-01-02")

		if iter.Before(notBefore) {
			continue
		}

		if !includeNotAfter && iter.Equal(notAfter) {
			continue
		}

		if iter.After(notAfter) {
			continue
		}

		if hd, ok := holidays[key]; ok && hd.Type == calendarv1.HolidayType_PUBLIC {
			// this is a public holiday

			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_HOLIDAY,
			})

			continue
		} else if ok {
			log.L(ctx).Infof("found holiday on %s with type %s", iter, hd.Type.String())
		}

		switch iter.Weekday() {
		case time.Saturday, time.Sunday: // FIXME(ppacher): should we make saturday configurable?
			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_WEEKEND,
			})
		default:
			dayTypes = append(dayTypes, &rosterv1.Day{
				Date: key,
				Type: rosterv1.DayType_DAY_TYPE_WORKDAY,
			})

			numberOfWorkingDays++
		}
	}

	return numberOfWorkingDays, dayTypes
}

type rosterShiftSlice struct {
	shifts []structs.PlannedShift
	defs   map[string]structs.WorkShift
}

func (rss *rosterShiftSlice) Len() int { return len(rss.shifts) }
func (rss *rosterShiftSlice) Less(i, j int) bool {
	si := rss.shifts[i]
	sj := rss.shifts[j]

	defI := rss.defs[si.WorkShiftID.Hex()]
	defJ := rss.defs[sj.WorkShiftID.Hex()]

	return defI.Order < defJ.Order
}
func (rss *rosterShiftSlice) Swap(i, j int) {
	rss.shifts[i], rss.shifts[j] = rss.shifts[j], rss.shifts[i]
}
