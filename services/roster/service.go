package roster

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/structs"
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

		if _, ok := workShiftLm[shift.WorkShiftId]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("work shift with id %q is not allowed for roster type %q", shift.WorkShiftId, rosterType.UniqueName))
		}

		roster.Shifts[idx] = conv
	}

	if roster.IsApproved() {
		// reset approval fields
		roster.Approved = false
		roster.ApprovedAt = time.Time{}
		roster.ApproverUserId = ""

		oldRosterID := roster.ID

		// generate a new id for the roster
		roster.ID = primitive.NewObjectID()

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
	analysis, err := svc.analyzeWorkTime(ctx, users, roster.From, roster.To, req.Msg.TimeTrackingOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate work-time: %w", err)
	}

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
	analysis, err := svc.analyzeWorkTime(ctx, allUserIds, roster.From, roster.To, true)
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
	t := time.Now()

	if req.Msg.Time.IsValid() {
		t = req.Msg.Time.AsTime()
	}

	if req.Msg.OnCall {
		if req.Msg.RosterTypeName == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("on_call may only be set if roster_type_name is set"))
		}
	}

	rosters, err := svc.Datastore.DutyRostersByTime(ctx, t)
	if err != nil {
		return nil, err
	}

	if len(rosters) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find any rosters for %s", t))
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

	userIds := make(map[string]struct{})
	for _, roster := range rosters {
		if rosterType.UniqueName != "" && roster.RosterTypeName != rosterType.UniqueName {
			continue
		}

		log.L(ctx).Debugf("checking roster %s (from %s to %s) with type %s", roster.ID.Hex(), roster.From, roster.To, roster.RosterTypeName)

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

			if !isAllowed {
				log.L(ctx).Debugf("shift %s is filtered. shift-tags=%v rosterType.shiftTags=%v rosterType.onCallTags=%v", shift.WorkShiftID, def.Tags, rosterType.ShiftTags, rosterType.OnCallTags)

				continue
			}

			if (shift.From.Before(t) || shift.From.Equal(t)) && (shift.To.After(t) || shift.To.Equal(t)) {
				relevantShifts = append(relevantShifts, shift)
				relevantRosters[roster.ID.Hex()] = struct{}{}
				for _, user := range shift.AssignedUserIds {
					userIds[user] = struct{}{}
				}
			} else {
				log.L(ctx).Debugf("shift is either before or after the requested time")
			}
		}
	}

	response := &rosterv1.GetWorkingStaffResponse{}
	for user := range userIds {
		response.UserIds = append(response.UserIds, user)
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
		dutyRoster []structs.DutyRoster
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

	var (
		from time.Time
		to   time.Time
	)

	response := &rosterv1.GetRosterResponse{
		Roster: make([]*rosterv1.Roster, len(dutyRoster)),
	}
	for idx, r := range dutyRoster {
		response.Roster[idx] = r.ToProto()

		rosterFrom := r.FromTime()
		rosterTo := r.ToTime()

		if from.IsZero() || rosterFrom.Before(from) {
			from = rosterFrom
		}

		if to.IsZero() || rosterTo.After(to) {
			to = rosterTo
		}
	}

	if shouldIncludeAnalysis && !from.IsZero() && !to.IsZero() {
		allUserIds, err := svc.FetchAllUserIds(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get user ids: %w", err)
		}

		// caculate the work-time for the roster
		analysis, err := svc.analyzeWorkTime(ctx, allUserIds, from.Format("2006-01-02"), to.Format("2006-01-02"), req.Msg.TimeTrackingOnly)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate work-time: %w", err)
		}

		response.WorkTimeAnalysis = analysis
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
