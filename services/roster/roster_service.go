package roster

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/bufbuild/connect-go"
	"github.com/mennanov/fmutils"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
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

func (svc *RosterService) CreateRosterType(ctx context.Context, req *connect.Request[rosterv1.CreateRosterTypeRequest]) (*connect.Response[rosterv1.CreateRosterTypeResponse], error) {
	model := structs.RosterType{
		UniqueName: req.Msg.RosterType.UniqueName,
		ShiftTags:  req.Msg.RosterType.ShiftTags,
		OnCallTags: req.Msg.RosterType.OnCallTags,
	}

	if err := svc.Datastore.SaveRosterType(ctx, model); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.CreateRosterTypeResponse{
		RosterType: model.ToProto(),
	}), nil
}

func (svc *RosterService) DeleteRosterType(ctx context.Context, req *connect.Request[rosterv1.DeleteRosterTypeRequest]) (*connect.Response[rosterv1.DeleteRosterTypeResponse], error) {
	if err := svc.Datastore.DeleteRosterType(ctx, req.Msg.UniqueName); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, err
	}

	return connect.NewResponse(&rosterv1.DeleteRosterTypeResponse{}), nil
}

func (svc *RosterService) ListRosterTypes(ctx context.Context, req *connect.Request[rosterv1.ListRosterTypesRequest]) (*connect.Response[rosterv1.ListRosterTypesResponse], error) {
	models, err := svc.Datastore.GetRosterTypes(ctx)
	if err != nil {
		return nil, err
	}

	res := &rosterv1.ListRosterTypesResponse{
		RosterTypes: make([]*rosterv1.RosterType, len(models)),
	}

	for idx, m := range models {
		res.RosterTypes[idx] = m.ToProto()
	}

	return connect.NewResponse(res), nil
}

func (svc *RosterService) SaveRoster(ctx context.Context, req *connect.Request[rosterv1.SaveRosterRequest]) (*connect.Response[rosterv1.SaveRosterResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	var roster structs.DutyRoster

	if req.Msg.Id != "" {
		var err error

		roster, err = svc.Datastore.DutyRosterByID(ctx, req.Msg.Id)
		if err != nil {
			return nil, err
		}
	} else {
		roster = structs.DutyRoster{
			From:           req.Msg.From,
			To:             req.Msg.To,
			CreatedAt:      time.Now(),
			LastModifiedBy: remoteUser.ID,
			UpdatedAt:      time.Now(),
			ShiftTags:      req.Msg.ShiftTags,
			RosterTypeName: req.Msg.RosterTypeName,
		}
	}

	rosterType, err := svc.Datastore.GetRosterType(ctx, roster.RosterTypeName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find roster type with name %q", roster.RosterTypeName))
		}

		return nil, err
	}

	// load all required shift definitions for this roster
	_, definitions, err := svc.getRequiredShifts(ctx, roster.FromTime(), roster.ToTime(), nil, rosterType.ShiftTags)
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

	if _, err := svc.Datastore.SaveDutyRoster(ctx, &roster); err != nil {
		return nil, err
	}

	allUserIds, err := svc.FetchAllUserIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ids: %w", err)
	}

	// caculate the work-time for the roster
	analysis, err := svc.analyzeWorkTime(ctx, allUserIds, roster.FromTime(), roster.ToTime())
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

	requiredShifts, definitions, err := svc.getRequiredShifts(ctx, from, to, nil, rosterType.ShiftTags)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.GetRequiredShiftsResponse{
		RequiredShifts:       make([]*rosterv1.RequiredShift, len(requiredShifts)),
		WorkShiftDefinitions: make([]*rosterv1.WorkShift, len(definitions)),
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

	// If the roster was already approved and we "re-approve" it, make sure to recalculate the off-time
	// costs.
	if roster.IsApproved() {
		if err := svc.Datastore.DeleteOffTimeCostsByRoster(ctx, roster.ID.Hex()); err != nil {
			return nil, fmt.Errorf("failed to remove off-time costs bound to the roster: %w", err)
		}
	}

	allUserIds, err := svc.FetchAllUserIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ids: %w", err)
	}

	fromTime := roster.FromTime()
	toTime := roster.ToTime()

	// caculate the work-time for the roster
	analysis, err := svc.analyzeWorkTime(ctx, allUserIds, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate work-time: %w", err)
	}

	approver := remoteUser.ID
	for _, an := range analysis {
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
		} else {
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
				return nil, fmt.Errorf("invalid off-time cost split, time-off=%q, vacation=%q, sum must not be more than %q", timeOffCosts, vacationCosts, diff)
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

	go func() {
		ctx := log.WithLogger(context.Background(), log.L(ctx))

		_, err = svc.sendRosterNotification(ctx, remoteUser.ID, roster, false)
		if err != nil {
			log.L(ctx).Errorf("failed to send roster notification: %s", err)
		}
	}()

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

		log.L(ctx).Infof("checking roster %s (from %s to %s) with type %s", roster.ID.Hex(), roster.From, roster.To, roster.RosterTypeName)

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
				log.L(ctx).Infof("shift %s is filtered. shift-tags=%v rosterType.shiftTags=%v rosterType.onCallTags=%v", shift.WorkShiftID, def.Tags, rosterType.ShiftTags, rosterType.OnCallTags)

				continue
			}

			if (shift.From.Before(t) || shift.From.Equal(t)) && (shift.To.After(t) || shift.To.Equal(t)) {
				relevantShifts = append(relevantShifts, shift)
				relevantRosters[roster.ID.Hex()] = struct{}{}
				for _, user := range shift.AssignedUserIds {
					userIds[user] = struct{}{}
				}
			} else {
				log.L(ctx).Infof("shift is either before or after the requested time")
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
		analysis, err := svc.analyzeWorkTime(ctx, allUserIds, from, to)
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

func (svc *RosterService) AnalyzeWorkTime(ctx context.Context, req *connect.Request[rosterv1.AnalyzeWorkTimeRequest]) (*connect.Response[rosterv1.AnalyzeWorkTimeResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing remote user"))
	}

	from, err := time.ParseInLocation("2006-01-02", req.Msg.From, time.Local)
	if err != nil {
		return nil, err
	}

	to, err := time.ParseInLocation("2006-01-02", req.Msg.To, time.Local)
	if err != nil {
		return nil, err
	}
	to = to.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	var userIds []string
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

	res, err := svc.analyzeWorkTime(ctx, userIds, from, to)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.AnalyzeWorkTimeResponse{
		Results: res,
	}), nil
}

func (svc *RosterService) SendRosterPreview(ctx context.Context, req *connect.Request[rosterv1.SendRosterPreviewRequest]) (*connect.Response[rosterv1.SendRosterPreviewResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	roster, err := svc.Datastore.DutyRosterByID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to load roster with id %q", req.Msg.Id))
		}

		return nil, err
	}

	deliveries, err := svc.sendRosterNotification(ctx, remoteUser.ID, roster, true)
	if err != nil {
		return nil, err
	}

	res := connect.NewResponse(&rosterv1.SendRosterPreviewResponse{
		Delivery: deliveries,
	})

	return res, nil
}

type event struct {
	from  time.Time
	to    time.Time
	name  string
	users []*idmv1.Profile
}

func (e event) id() string {
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("%s-%s-%s-%s", e.name, e.from, e.to, time.Now())))

	return hex.EncodeToString(h.Sum(nil))
}

type calendar struct {
	events []event
}

func (c calendar) ToICS(rosterFrom time.Time) string {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodAdd)
	cal.SetProductId("-//dobersberg.vet//Tierklinik Dobersberg 2023c//EN")
	cal.SetName("Dienstplan " + rosterFrom.Format("01/2006"))
	cal.SetTzid("Europe/Vienna")

	seq := 1
	dtTime := time.Now()
	for _, e := range c.events {
		evt := cal.AddEvent(e.id())
		evt.SetStartAt(e.from)
		evt.SetEndAt(e.to)
		evt.SetSummary(e.name)
		evt.SetDtStampTime(dtTime)
		evt.SetOrganizer("office@tierklinikdobersberg.at", ics.WithCN("Tierklinik Dobersberg"))

		for _, user := range e.users {
			userDisplayName := user.User.DisplayName
			if userDisplayName == "" {
				userDisplayName = user.User.Username
			}
			userPrimaryMail := ""
			if user.User.PrimaryMail != nil {
				userPrimaryMail = user.User.PrimaryMail.Address
			}

			evt.AddAttendee(userPrimaryMail, ics.WithCN(userDisplayName), ics.ParticipationRoleReqParticipant, ics.ParticipationStatusAccepted)
		}

		evt.SetSequence(seq)
	}

	blob := cal.Serialize()

	return blob
}

func (svc *RosterService) sendRosterNotification(ctx context.Context, senderId string, roster structs.DutyRoster, isPreview bool) ([]*idmv1.DeliveryNotification, error) {
	type Shift struct {
		Name string
		From string
		To   string
	}

	var (
		perUserShifts = make(map[string]map[string][]Shift)
		targetUsers   = make(map[string]*idmv1.Profile)
	)

	calendar := new(calendar)

	workShifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load work-shift definitions: %w", err)
	}

	wsLm := data.IndexSlice(workShifts, func(e structs.WorkShift) string { return e.ID.Hex() })

	allUsers, err := svc.FetchAllUserProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all user profiles: %w", err)
	}

	userLm := data.IndexSlice(allUsers, func(u *idmv1.Profile) string { return u.GetUser().GetId() })

	for _, shift := range roster.Shifts {
		shiftName := wsLm[shift.WorkShiftID.Hex()].Name

		event := event{
			name: shiftName,
			from: shift.From,
			to:   shift.To,
		}

		for _, usrId := range shift.AssignedUserIds {
			targetUsers[usrId] = userLm[usrId]

			event.users = append(event.users, userLm[usrId])

			shiftDate := shift.From.Format("2006-01-02")

			if perUserShifts[usrId] == nil {
				perUserShifts[usrId] = make(map[string][]Shift)
			}

			perUserShifts[usrId][shiftDate] = append(perUserShifts[usrId][shiftDate], Shift{
				Name: shiftName,
				From: shift.From.Format(time.RFC3339),
				To:   shift.To.Format(time.RFC3339),
			})
		}

		calendar.events = append(calendar.events, event)
	}

	var (
		userDiff     map[string][]ShiftDiff
		isSuperseded bool
	)

	if oldRoster, err := svc.Datastore.GetSupersededDutyRoster(ctx, roster.ID); err == nil {
		userDiff, err = diffRosters(ctx, oldRoster, &roster)
		if err != nil {
			log.L(ctx).Errorf("failed to diff duty rosters: %s", err)
		} else {
			isSuperseded = true
		}
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		// just log the error, we're going to send the normal duty roster
		// notification anyway
		log.L(ctx).Errorf("failed to load superseded duty roster: %s", err)
	}

	// make sure every user that has a diff is also part of the target users.
	for userId := range userDiff {
		targetUsers[userId] = userLm[userId]
	}

	userIds := maps.Keys(targetUsers)

	workTime, err := svc.analyzeWorkTime(ctx, userIds, roster.FromTime(), roster.ToTime())
	if err != nil {
		return nil, fmt.Errorf("failed to analyze work time: %w", err)
	}

	perUserCtx := make(map[string]*structpb.Struct, len(userIds))

	for _, userId := range userIds {
		workingDates := perUserShifts[userId]
		var userWorkTime *rosterv1.WorkTimeAnalysis

		for _, wt := range workTime {
			if wt.UserId == userId {
				userWorkTime = wt

				break
			}
		}

		diffMaps := make([]any, 0, len(userDiff[userId]))
		for _, shift := range userDiff[userId] {
			var m map[string]any
			if err := mapstructure.Decode(shift, &m); err != nil {
				return nil, fmt.Errorf("failed to convert shift-diff to map: %w", err)
			}

			m["Name"] = wsLm[shift.ID].Name

			diffMaps = append(diffMaps, m)
		}

		shiftMaps := make(map[string]any, len(workingDates))
		for date, shifts := range workingDates {
			result := make([]any, len(shifts))
			for idx, shift := range shifts {
				var shiftMap map[string]any
				if err := mapstructure.Decode(shift, &shiftMap); err != nil {
					return nil, fmt.Errorf("failed to convert shift to map: %w", err)
				}

				result[idx] = shiftMap
			}

			shiftMaps[date] = result
		}

		tmplCtx := map[string]any{
			"Dates":        shiftMaps,
			"ExpectedTime": int64(userWorkTime.ExpectedTime.AsDuration().Seconds()),
			"PlannedTime":  int64(userWorkTime.PlannedTime.AsDuration().Seconds()),
			"Overtime":     int64(userWorkTime.Overtime.AsDuration().Seconds()),
			"Preview":      isPreview,
			"RosterDate":   roster.FromTime().Format("2006/01"),
			"RosterURL":    fmt.Sprintf(svc.Config.PreviewRosterURL, roster.ID.Hex()),
			"From":         roster.From,
			"To":           roster.To,
			"Diff":         diffMaps,
			"Superseded":   isSuperseded,
		}

		s, err := structpb.NewStruct(tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("failed prepare structpb: %w", err)
		}

		perUserCtx[userId] = s
	}

	subject := fmt.Sprintf("Dienstplan für %s", roster.FromTime().Format("2006/01"))
	if isPreview {
		subject = fmt.Sprintf("Vorläufiger Dienstplan für %s", roster.FromTime().Format("2006/01"))
	}

	templateBody, err := fs.ReadFile(svc.Templates, "mails/dist/roster-notification.html")
	if err != nil {
		return nil, err
	}

	email := &idmv1.EMailMessage{
		Subject:     subject,
		Body:        string(templateBody),
		Attachments: []*idmv1.Attachment{},
	}

	if !isPreview {
		email.Attachments = append(email.Attachments, &idmv1.Attachment{
			Name:           "Dienstplan.ics",
			MediaType:      "text/calendar; method=ADD; name=Dienstplan.ics",
			Content:        []byte(calendar.ToICS(roster.FromTime())),
			AttachmentType: idmv1.AttachmentType_ATTACHEMNT,
			ContentId:      "Dienstplan.ics",
		})
	}

	req := &idmv1.SendNotificationRequest{
		TargetUsers:            userIds,
		PerUserTemplateContext: perUserCtx,
		SenderUserId:           senderId,
		Message: &idmv1.SendNotificationRequest_Email{
			Email: email,
		},
	}

	res, err := svc.Notify.SendNotification(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	return res.Msg.Deliveries, nil
}

func (svc *RosterService) GetUserShifts(ctx context.Context, req *connect.Request[rosterv1.GetUserShiftsRequest]) (*connect.Response[rosterv1.GetUserShiftsResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	if req.Msg.Timerange == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("missing timerange"))
	}

	if !req.Msg.Timerange.From.IsValid() || !req.Msg.Timerange.To.IsValid() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid from or to time"))
	}

	from := req.Msg.Timerange.From.AsTime().Local()
	to := req.Msg.Timerange.To.AsTime().Local()

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
			if slices.Contains(shift.AssignedUserIds, remoteUser.ID) {
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

func (svc *RosterService) analyzeWorkTime(ctx context.Context, userIds []string, from, to time.Time) ([]*rosterv1.WorkTimeAnalysis, error) {
	log.L(ctx).Infof("analyzing work time for users between %s and %s", from, to)

	// fetch all distinct rosters
	distinctRosters := make(map[string]structs.DutyRoster)
	for iter := from; to.After(iter) || to.Equal(iter); iter = iter.AddDate(0, 0, 1) {
		rosters, err := svc.Datastore.DutyRostersByTime(ctx, iter)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch roster for %s: %w", iter, err)
		}

		for _, roster := range rosters {
			distinctRosters[roster.ID.Hex()] = roster
		}
	}

	log.L(ctx).Debugf("found %d distinct rosters that need to be analyzed", len(distinctRosters))

	// fetch all work shifts
	workShifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch work-shift definitions: %w", err)
	}

	workShiftLookupMap := make(map[string]structs.WorkShift, len(workShifts))
	for _, shift := range workShifts {
		workShiftLookupMap[shift.ID.Hex()] = shift
	}

	// small helper function to generate a string from ISOWeek.
	weekKey := func(d time.Time) string {
		year, week := d.ISOWeek()
		return fmt.Sprintf("%d-%02d", year, week)
	}

	// prepare some maps to hold our aggregated results.
	workTimes := make(map[string]time.Duration, len(userIds))
	workTimesPerWeek := make(map[string]map[string]time.Duration)
	workTimePerDay := make(map[string]map[string]time.Duration)
	overtimePerUser := make(map[string]time.Duration)

	expectedWorkTimes := make(map[string]time.Duration, len(userIds))
	results := make(map[string]*rosterv1.WorkTimeAnalysis)

	for _, userId := range userIds {
		workTimes[userId] = 0
		expectedWorkTimes[userId] = 0
		workTimesPerWeek[userId] = make(map[string]time.Duration)
		workTimePerDay[userId] = make(map[string]time.Duration)
		results[userId] = &rosterv1.WorkTimeAnalysis{
			UserId: userId,
		}
	}

	// actually calculate the planned work-time
	for _, roster := range distinctRosters {
		for _, shift := range roster.Shifts {
			// get the work-shift definition
			definition, ok := workShiftLookupMap[shift.WorkShiftID.Hex()]
			if !ok {
				return nil, fmt.Errorf("failed to find work-shift definition %s", shift.WorkShiftID.Hex())
			}

			// skip the shift if it's not within our time-range
			if shift.To.Before(from) {
				log.L(ctx).Debugf("skipping shift %s because of shift.To (%s) is before from (%s)", definition.Name, shift.To, from)
				continue
			}
			if shift.From.After(to) {
				log.L(ctx).Debugf("skipping shift %s because of shift.From (%s) is after to (%s)", definition.Name, shift.From, to)
				continue
			}

			// find out how much time the shift is worth
			var timeWorth time.Duration
			if definition.MinutesWorth != nil && *definition.MinutesWorth > 0 { // FIXME(ppacher): remove the > 0 check so it's possible a shift is nothing worth
				log.L(ctx).Debugf("shift %s has an explicit time-worth field set to %d minutes", shift.WorkShiftID.Hex(), *definition.MinutesWorth)
				timeWorth = time.Duration(*definition.MinutesWorth) * time.Minute
			} else {
				timeWorth = shift.To.Sub(shift.From)
				log.L(ctx).Debugf("shift %s is %s worth", shift.WorkShiftID.Hex(), timeWorth)
			}

			for _, staff := range shift.AssignedUserIds {
				_, ok := workTimes[staff]
				if !ok {
					// skip this user if it wasn't requested.
					log.L(ctx).Debugf("skipping planned work-time analysis for user %s", staff)
					continue
				}

				log.L(ctx).Infof("user %s: %s for shift %s (%s) %s to %s", staff, timeWorth, definition.Name, shift.WorkShiftID.Hex(), shift.From, shift.To)

				workTimes[staff] += timeWorth
				workTimesPerWeek[staff][weekKey(shift.From)] += timeWorth
				workTimePerDay[staff][shift.From.Format("2006-01-02")] += timeWorth
			}
		}
	}

	// get the number of working-days
	holidays, err := svc.getHolidayLookupMap(ctx, from, to)
	if err != nil {
		return nil, err
	}

	// calculate the expected work times
	for _, userId := range userIds {
		workTimeHistory, err := svc.Datastore.WorkTimeHistoryForStaff(ctx, userId)
		if err != nil {
			return nil, fmt.Errorf("failed to get work-time history for user %s: %w", userId, err)
		}

		startTime := from
		for idx, wt := range workTimeHistory {
			// find out until when the workTimeHistory is effective.
			until := to
			includeUntil := true

			if idx < len(workTimeHistory)-1 {
				until = workTimeHistory[idx+1].ApplicableFrom
				includeUntil = false
			}

			if until.After(to) {
				until = to
				includeUntil = true
			}

			// skip this entry if it either gets in effect after our requested time period
			// or if it's no in effect anymore.
			if wt.ApplicableFrom.After(to) || until.Before(from) {
				// not applicable
				continue
			}

			analysis := &rosterv1.WorkTimeAnalysisStep{
				WorkTimeId:      wt.ID.Hex(),
				WorkTimePerWeek: durationpb.New(wt.TimePerWeek),
				From:            startTime.Format("2006-01-02"),
				To:              until.Format("2006-01-02"),
			}

			// build a map to get the number of work-days per week and month
			weekWorkDays := make(map[string]struct {
				Year int
				Week int
				Days int
			})
			monthWorkDays := make(map[string]int)

			for iter := startTime; iter.Before(until) || (includeUntil && iter.Equal(until)); iter = iter.AddDate(0, 0, 1) {
				if hd, ok := holidays[iter.Format("2006-01-02")]; ok && hd.Type == calendarv1.HolidayType_PUBLIC {
					// this is a public holiday
					continue
				} else if ok {
					log.L(ctx).Infof("found holiday on %s with type %s", iter, hd.Type.String())
				}

				switch iter.Weekday() {
				case time.Saturday, time.Sunday: // FIXME(ppacher): should we make saturday configurable?
				default:
					key := weekKey(iter)
					val := weekWorkDays[key]

					if val.Year == 0 {
						year, week := iter.ISOWeek()
						val.Year = year
						val.Week = week
					}

					val.Days++
					weekWorkDays[key] = val

					monthWorkDays[iter.Format("2006-01")]++
				}
			}

			var (
				expectedWork time.Duration
				sumPlanned   time.Duration
			)

			for weekKey, week := range weekWorkDays {
				ratio := float64(week.Days) / 5.0
				d := time.Duration(float64(wt.TimePerWeek) * ratio)

				expectedWorkTimes[userId] += d
				expectedWork += d

				sumPlanned += workTimesPerWeek[userId][weekKey]
				analysis.Weeks = append(analysis.Weeks, &rosterv1.WorkTimeAnalysisWeek{
					Year:         int32(week.Year),
					Week:         int32(week.Week),
					WorkingDays:  int32(week.Days),
					ExpectedWork: durationpb.New(d),
					Planned:      durationpb.New(workTimesPerWeek[userId][weekKey]),
				})
			}

			// calculate the work time per month
			worktimePerMonth := make(map[string]time.Duration)
			for dayStr, plannedWorkTime := range workTimePerDay[userId] {
				dayTime, _ := time.ParseInLocation("2006-01-02", dayStr, time.Local)
				month := dayTime.Format("2006-01")

				if dayTime.Before(startTime) || dayTime.After(until) {
					continue
				}

				worktimePerMonth[month] += plannedWorkTime
			}

			// calculate the expected time per month
			expectedTimePerMonth := make(map[string]time.Duration)
			for month, numberOfWorkingDays := range monthWorkDays {
				ratio := float64(numberOfWorkingDays) / 5.0
				d := time.Duration(float64(wt.TimePerWeek) * ratio)

				expectedTimePerMonth[month] += d
			}

			// calculate overtime
			var overtime time.Duration
			for month, expectedTime := range expectedTimePerMonth {
				plannedTime := worktimePerMonth[month]

				monthTime, _ := time.ParseInLocation("2006-01", month, time.Local)

				daysInMonth := daysIn(ctx, holidays, monthTime.Month(), monthTime.Year(), startTime, until, includeUntil)
				overtimeRatio := (wt.OvertimeAllowancePerMonth / time.Duration(daysInMonth)) * time.Duration(monthWorkDays[month])

				diff := plannedTime - expectedTime
				if diff > 0 {
					diff = diff - overtimeRatio
					if diff < 0 {
						diff = 0
					}
				}

				overtime += diff
				overtimePerUser[userId] += diff

				log.L(ctx).WithFields(logrus.Fields{
					"month":                    month,
					"expectedTimePerMonth":     expectedTime,
					"plannedTime":              plannedTime,
					"totalWorkingDaysInMonth":  daysInMonth,
					"rosterWorkingDaysInMonth": monthWorkDays[month],
					"overtimeRatio":            overtimeRatio,
					"overtime":                 diff,
				}).Infof("analyzed overtime per month")
			}

			sort.Sort(sortWeeks(analysis.Weeks))

			analysis.ExpectedWorkTime = durationpb.New(expectedWork)
			analysis.Planned = durationpb.New(sumPlanned)
			analysis.Overtime = durationpb.New(overtime)

			results[userId].Steps = append(results[userId].Steps, analysis)

			startTime = until
		}
	}

	for _, userId := range userIds {
		results[userId].ExpectedTime = durationpb.New(expectedWorkTimes[userId])
		results[userId].PlannedTime = durationpb.New(workTimes[userId])
		results[userId].Overtime = durationpb.New(overtimePerUser[userId])
	}

	var resultSlice = make([]*rosterv1.WorkTimeAnalysis, 0, len(results))
	for _, val := range results {
		resultSlice = append(resultSlice, val)
	}

	return resultSlice, nil
}

type sortWeeks []*rosterv1.WorkTimeAnalysisWeek

func (sw sortWeeks) Len() int { return len(sw) }
func (sw sortWeeks) Less(i, j int) bool {
	if sw[i].Year < sw[j].Year {
		return true
	}

	if sw[i].Year > sw[j].Year {
		return false
	}

	return sw[i].Week < sw[j].Week
}
func (sw sortWeeks) Swap(i, j int) { sw[i], sw[j] = sw[j], sw[i] }

func (svc *RosterService) getRequiredShifts(ctx context.Context, from, to time.Time, profiles *[]*idmv1.Profile, allowedTags []string) ([]structs.RequiredShift, []structs.WorkShift, error) {
	// fetch user profiles
	if profiles == nil {
		profiles = new([]*idmv1.Profile)
	}
	if len(*profiles) == 0 {
		var err error
		*profiles, err = svc.FetchAllUserProfiles(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch users: %w", err)
		}
	}

	// fetch all holidays
	holidays, err := svc.getHolidayLookupMap(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	// generate a list of required shifts
	var (
		results          = make([]structs.RequiredShift, 0)
		shiftLm          = make(map[string]struct{})
		shiftDefinitions = make([]structs.WorkShift, 0)
	)

	for iter := from; to.After(iter) || to.Equal(iter); iter = iter.AddDate(0, 0, 1) {
		_, isHoliday := holidays[iter.Format("2006-01-02")]

		shiftsPerDay, err := svc.Datastore.GetShiftsForDay(ctx, iter.Weekday(), isHoliday)
		if err != nil {
			return nil, nil, err
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
				hasRole := data.SliceOverlapsFunc(shift.EligibleRoles, profile.Roles, func(role *idmv1.Role) string {
					return role.Id
				})

				if !hasRole {
					continue
				}

				// check approved off-time requests
				approved := true
				offTimeRequests, err := svc.Datastore.FindOffTimeRequests(ctx, shiftStart, shiftEnd, &approved, []string{profile.User.Id})
				if err != nil {
					return nil, nil, fmt.Errorf("failed to load approved off-time requests for user %s: %w", profile.User.Id, err)
				}

				var violations []*rosterv1.ConstraintViolation
				// create a "fake" violation for each approved off-time-request
				for _, offReq := range offTimeRequests {
					violations = append(violations, &rosterv1.ConstraintViolation{
						Hard: true,
						Kind: &rosterv1.ConstraintViolation_OffTime{
							OffTime: &rosterv1.OffTimeViolation{
								Entry: offReq.ToProto(),
							},
						},
					})
				}

				// check if the user is eligible or not
				if len(violations) == 0 {
					requiredShift.EligibleUserIds = append(requiredShift.EligibleUserIds, profile.User.Id)
				} else {
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

	return results, shiftDefinitions, nil
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

func daysIn(ctx context.Context, holidays map[string]*calendarv1.PublicHoliday, m time.Month, year int, notBefore, notAfter time.Time, includeNotAfter bool) int {
	firstDayInMonth := time.Date(year, m, 1, 0, 0, 0, 0, time.Local)

	var days int
	for iter := firstDayInMonth; iter.Month() == m; iter = iter.AddDate(0, 0, 1) {
		if hd, ok := holidays[iter.Format("2006-01-02")]; ok && hd.Type == calendarv1.HolidayType_PUBLIC {
			// this is a public holiday
			continue
		} else if ok {
			log.L(ctx).Infof("found holiday on %s with type %s", iter, hd.Type.String())
		}

		if iter.Before(notBefore) {
			continue
		}

		if !includeNotAfter && iter.Equal(notAfter) {
			continue
		}

		if iter.After(notAfter) {
			continue
		}

		switch iter.Weekday() {
		case time.Saturday, time.Sunday: // FIXME(ppacher): should we make saturday configurable?
		default:
			days++
		}
	}

	return days
}

type ShiftDiff struct {
	ID   string
	From string
	To   string

	// If assigned is true than the user has been assigned to
	// this shift.
	// If assigned is false, the user has been removed from this
	// shift.
	Assigned bool
}

func diffRosters(ctx context.Context, old, new *structs.DutyRoster) (map[string] /*userId*/ []ShiftDiff, error) {
	if old.From != new.From || old.To != new.To {
		return nil, fmt.Errorf("cannot diff rosters with different from/to times")
	}

	if old.SupersededBy.String() != new.ID.String() {
		return nil, fmt.Errorf("can only diff rosters where one superseded the other")
	}

	result := make(map[string][]ShiftDiff)

	plannedShiftKey := func(p structs.PlannedShift) string {
		return fmt.Sprintf("%s/%s/%s", p.WorkShiftID, p.From.Format(time.RFC3339), p.To.Format(time.RFC3339))
	}

	// convert our planned shifts to a lookup map
	oldShifts := data.IndexSlice(old.Shifts, plannedShiftKey)
	newShifts := data.IndexSlice(new.Shifts, plannedShiftKey)

	// iterate over all "newShifts" and check if a user has been assigned/removed from the related oldShifts
	for shiftID, shift := range newShifts {
		oldShift, ok := oldShifts[shiftID]

		if !ok {
			// this shift has not even been planned in the old roster
			// so add an assignment for all users of this shift

			for _, userId := range shift.AssignedUserIds {
				result[userId] = append(result[userId], ShiftDiff{
					ID:       shift.WorkShiftID.Hex(),
					From:     shift.From.Format(time.RFC3339),
					To:       shift.To.Format(time.RFC3339),
					Assigned: true,
				})
			}

			continue
		}

		// check for new assignments
		for _, userId := range shift.AssignedUserIds {
			if !slices.Contains(oldShift.AssignedUserIds, userId) {
				// this user has been assigned
				result[userId] = append(result[userId], ShiftDiff{
					ID:       shift.WorkShiftID.Hex(),
					From:     shift.From.Format(time.RFC3339),
					To:       shift.To.Format(time.RFC3339),
					Assigned: true,
				})
			}
		}

		// check for new unassignments
		for _, userId := range oldShift.AssignedUserIds {
			if !slices.Contains(shift.AssignedUserIds, userId) {
				// this user has been unassigned
				result[userId] = append(result[userId], ShiftDiff{
					ID:       shift.WorkShiftID.Hex(),
					From:     shift.From.Format(time.RFC3339),
					To:       shift.To.Format(time.RFC3339),
					Assigned: false,
				})
			}
		}

		// delete the shift from the oldShifts map
		delete(oldShifts, shiftID)
	}

	// check which shifts has been planned but got removed
	for _, shift := range oldShifts {
		for _, userId := range shift.AssignedUserIds {
			result[userId] = append(result[userId], ShiftDiff{
				ID:       shift.WorkShiftID.Hex(),
				From:     shift.From.Format(time.RFC3339),
				To:       shift.To.Format(time.RFC3339),
				Assigned: true,
			})
		}
	}

	return result, nil
}
