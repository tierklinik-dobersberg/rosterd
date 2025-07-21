package worktime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/hashicorp/go-multierror"
	"github.com/mennanov/fmutils"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/config"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	rosterv1connect.UnimplementedWorkTimeServiceHandler

	*config.Providers
}

func New(p *config.Providers) *Service {
	return &Service{
		Providers: p,
	}
}

func (svc *Service) SetWorkTime(ctx context.Context, req *connect.Request[rosterv1.SetWorkTimeRequest]) (*connect.Response[rosterv1.SetWorkTimeResponse], error) {
	var merr = new(multierror.Error)
	var response = &rosterv1.SetWorkTimeResponse{
		WorkTimes: make([]*rosterv1.WorkTime, len(req.Msg.WorkTimes)),
	}

	for idx, wt := range req.Msg.WorkTimes {
		applicableFrom, err := time.ParseInLocation("2006-01-02", wt.ApplicableAfter, time.Local)
		if err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("invalid value for field 'applicable_after': %w", err))
		}

		model := structs.WorkTime{
			UserID:                    wt.UserId,
			TimePerWeek:               wt.TimePerWeek.AsDuration(),
			ApplicableFrom:            applicableFrom,
			VacationWeeksPerYear:      wt.VacationWeeksPerYear,
			OvertimeAllowancePerMonth: wt.OvertimeAllowancePerMonth.AsDuration(),
			ExcludeFromTimeTracking:   wt.ExcludeFromTimeTracking,
		}

		if wt.EndsWith != "" {
			var err error
			model.EndsWith, err = time.ParseInLocation("2006-01-02", wt.EndsWith, time.Local)

			if err != nil {
				merr.Errors = append(merr.Errors, fmt.Errorf("invalid value for field 'ends_with': %w", err))
			}
		}

		if !wt.OvertimeAllowancePerMonth.IsValid() {
			model.OvertimeAllowancePerMonth = 0
		}

		// validate that the user actually exists.
		if err := svc.VerifyUserExists(ctx, wt.UserId); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("user_id %q: failed to fetch user record: %w", wt.UserId, err))

			continue
		}

		// finally store the work-time record in the database.
		if err := svc.Datastore.SaveWorkTimePerWeek(ctx, &model); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("user_id %q: %w", wt.UserId, err))
		}

		log.L(ctx).Info("updated work time for user", "userId", model.UserID, "timePerWeek", model.TimePerWeek, "applicableFrom", model.ApplicableFrom)

		response.WorkTimes[idx] = worktimeToProto(model)
	}

	if err := merr.ErrorOrNil(); err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) GetWorkTime(ctx context.Context, req *connect.Request[rosterv1.GetWorkTimeRequest]) (*connect.Response[rosterv1.GetWorkTimeResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// determine the read_mask to apply
	paths := []string{
		"results.user_id",
		"results.current",
		"results.history",
	}
	if req.Msg.ReadMask != nil && len(req.Msg.ReadMask.Paths) > 0 {
		paths = req.Msg.ReadMask.Paths
	}

	// determine for which users we should load the work-times
	userIds := req.Msg.UserIds
	if len(userIds) == 0 {
		if remoteUser.Admin {
			var err error
			userIds, err = svc.FetchAllUserIds(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch user ids: %w", err)
			}

		} else {
			userIds = []string{remoteUser.ID}
		}
	} else {
		if !remoteUser.Admin && (len(userIds) != 1 || userIds[0] != remoteUser.ID) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you're not allowed to perform this operation"))
		}
	}

	// determine which fields we should populate.
	shouldLoadHistory := false
	shouldLoadCurrent := false
	for _, p := range paths {
		if p == "results" {
			shouldLoadHistory = true
			shouldLoadCurrent = true

			break
		}

		if strings.HasPrefix(p, "results.history") {
			shouldLoadHistory = true
		}

		if strings.HasPrefix(p, "results.current") {
			shouldLoadCurrent = true
		}
	}

	// acutally prepare the response
	response := &rosterv1.GetWorkTimeResponse{
		Results: make([]*rosterv1.UserWorkTime, len(userIds)),
	}

	// load the current work-time if requested:
	var current map[string]structs.WorkTime
	if shouldLoadCurrent {
		var err error
		current, err = svc.Datastore.GetCurrentWorkTimes(ctx, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to load current work times: %w", err)
		}
	}

	for idx, userId := range userIds {
		userWorkTime := &rosterv1.UserWorkTime{
			UserId: userId,
		}

		// load the work time history if requested.
		if shouldLoadHistory {
			history, err := svc.Datastore.WorkTimeHistoryForStaff(ctx, userId)
			if err == nil {
				userWorkTime.History = make([]*rosterv1.WorkTime, len(history))

				for hIdx, wt := range history {
					userWorkTime.History[hIdx] = worktimeToProto(wt)
				}
			} else {
				log.L(ctx).Error("failed to load work-time history for user", "userId", userId, "error", err)
			}
		}

		if shouldLoadCurrent {
			if wt, ok := current[userId]; ok {
				userWorkTime.Current = worktimeToProto(wt)
			}
		}

		response.Results[idx] = userWorkTime
	}

	if req.Msg.ReadMask != nil && len(req.Msg.ReadMask.Paths) > 0 {
		fmutils.Filter(response, req.Msg.ReadMask.Paths)
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) DeleteWorkTime(ctx context.Context, req *connect.Request[rosterv1.DeleteWorkTimeRequest]) (*connect.Response[rosterv1.DeleteWorkTimeResponse], error) {
	if err := svc.Datastore.DeleteWorkTime(ctx, req.Msg.Ids...); err != nil {
		return nil, err
	}

	return connect.NewResponse(new(rosterv1.DeleteWorkTimeResponse)), nil
}

func (svc *Service) GetVacationCreditsLeft(ctx context.Context, req *connect.Request[rosterv1.GetVacationCreditsLeftRequest]) (*connect.Response[rosterv1.GetVacationCreditsLeftResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// determine for which users we want to load costs
	var userIds []string
	if req.Msg.ForUsers != nil {
		hasOtherUsers := false

		for _, id := range req.Msg.ForUsers.UserIds {
			if id != remoteUser.ID {
				hasOtherUsers = true
				break
			}
		}

		if hasOtherUsers && !remoteUser.Admin {
			return nil, connect.NewError(connect.CodeAborted, fmt.Errorf("you're not allowed to perform this operation"))
		}

		userIds = req.Msg.ForUsers.UserIds
		if len(userIds) == 0 {
			if remoteUser.Admin {
				var err error
				userIds, err = svc.FetchAllUserIds(ctx)
				if err != nil {
					return nil, err
				}
			} else {
				userIds = []string{remoteUser.ID}
			}
		}
	} else {
		userIds = []string{remoteUser.ID}
	}

	costs, err := svc.Datastore.GetOffTimeCosts(ctx, userIds...)
	if err != nil {
		return nil, err
	}

	costsByUser := make(map[string][]structs.OffTimeCosts)
	for _, c := range costs {
		costsByUser[c.UserID] = append(costsByUser[c.UserID], c)
	}

	response := &rosterv1.GetVacationCreditsLeftResponse{
		Results: make([]*rosterv1.UserVacationSum, len(userIds)),
	}

	until := time.Now()
	if req.Msg.Until.IsValid() {
		until = req.Msg.Until.AsTime()
	}

	for idx, userId := range userIds {
		workHistory, err := svc.Datastore.WorkTimeHistoryForStaff(ctx, userId)
		if err != nil {
			return nil, err
		}

		perUser := &rosterv1.UserVacationSum{
			UserId:   userId,
			Analysis: &rosterv1.AnalyzeVacation{},
		}

		var (
			vacationSum time.Duration
			timeOffSum  time.Duration
		)

		// calculate the total amount of vacation hours
		for idx := 0; idx < len(workHistory); idx++ {
			iter := workHistory[idx]
			endsAt := until

			// skip this work-time entry if it becomes active after the
			// requested time-frame.
			if iter.ApplicableFrom.After(until) {
				continue
			}

			switch {
			case !iter.EndsWith.IsZero():
				endsAt = iter.EndsWith

			case idx+1 < len(workHistory):
				next := workHistory[idx+1]
				endsAt = next.ApplicableFrom
			}

			// if there's another work-history entry after this one we need
			// to update endsAt to the beginning of the next entry.

			daysUntilEnd := math.Floor(float64(endsAt.Sub(iter.ApplicableFrom)) / float64(time.Hour*24))

			vacationWeeksPerDay := float64(iter.VacationWeeksPerYear) / 365.0
			vacationsPerPeriod := vacationWeeksPerDay * float64(iter.TimePerWeek) * float64(daysUntilEnd)

			if !iter.ExcludeFromTimeTracking {
				vacationSum += time.Duration(vacationsPerPeriod)
			}

			if req.Msg.Analyze {
				sl := &rosterv1.AnalyzeVacationSum{
					WorkTime:            worktimeToProto(iter),
					EndsAt:              timestamppb.New(endsAt),
					NumberOfDays:        float64(daysUntilEnd),
					VacationWeeksPerDay: float64(vacationWeeksPerDay),
					VacationPerWorkTime: durationpb.New(time.Duration(vacationsPerPeriod)),
				}

				userCosts := costsByUser[userId]
				slSum := time.Duration(0)

				for _, cost := range userCosts {
					if !cost.IsVacation || cost.Date.After(endsAt) || cost.Date.Before(iter.ApplicableFrom) {
						continue
					}

					slSum += cost.Costs

					sl.Costs = append(sl.Costs, &rosterv1.OffTimeCosts{
						Id:         cost.ID.Hex(),
						OfftimeId:  cost.OfftimeID.Hex(),
						RosterId:   cost.RosterID.Hex(),
						UserId:     cost.UserID,
						CreatedAt:  timestamppb.New(cost.CreatedAt),
						CreatorId:  cost.CreatorId,
						Costs:      durationpb.New(cost.Costs),
						IsVacation: cost.IsVacation,
					})
				}

				sl.CostsSum = durationpb.New(slSum)
				perUser.Analysis.Slices = append(perUser.Analysis.Slices, sl)
			}

			// if this entry ends at or after the maximum time-frame
			// we can stop now.
			if endsAt.After(until) || endsAt.Equal(until) {
				break
			}
		}

		userCosts := costsByUser[userId]
		for _, cost := range userCosts {
			if !until.IsZero() && until.Before(cost.Date) {
				continue
			}

			if cost.IsVacation {
				vacationSum += cost.Costs // costs is negative
			} else {
				timeOffSum += cost.Costs
			}
		}

		perUser.VacationCreditsLeft = durationpb.New(vacationSum.Round(time.Minute))
		perUser.TimeOffCredits = durationpb.New(timeOffSum.Round(time.Minute))

		response.Results[idx] = perUser
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) UpdateWorkTime(ctx context.Context, req *connect.Request[rosterv1.UpdateWorkTimeRequest]) (*connect.Response[rosterv1.UpdateWorkTimeResponse], error) {
	wt, err := svc.Datastore.GetWorktimeByID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no work-time with the given id"))
		}

		return nil, err
	}

	paths := []string{
		"ends_with",
		"exclude_from_time_tracking",
	}

	if p := req.Msg.GetFieldMask().GetPaths(); len(p) > 0 {
		paths = p
	}

	for _, p := range paths {
		switch p {
		case "ends_with":
			if req.Msg.EndsWith != "" {
				var err error
				wt.EndsWith, err = time.ParseInLocation("2006-01-02", req.Msg.EndsWith, time.Local)
				if err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid value for field 'ends_with': %w", err))
				}
			} else {
				wt.EndsWith = time.Time{}
			}

		case "exclude_from_time_tracking":
			wt.ExcludeFromTimeTracking = req.Msg.ExcludeFromTimeTracking

		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid path in field_mask"))
		}
	}

	if err := svc.Datastore.UpdateWorkTime(ctx, wt); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.UpdateWorkTimeResponse{
		Worktime: worktimeToProto(*wt),
	}), nil
}

func worktimeToProto(wt structs.WorkTime) *rosterv1.WorkTime {
	wtpb := &rosterv1.WorkTime{
		Id:                        wt.ID.Hex(),
		UserId:                    wt.UserID,
		TimePerWeek:               durationpb.New(wt.TimePerWeek),
		ApplicableAfter:           wt.ApplicableFrom.Local().Format("2006-01-02"),
		VacationWeeksPerYear:      wt.VacationWeeksPerYear,
		OvertimeAllowancePerMonth: durationpb.New(wt.OvertimeAllowancePerMonth),
		ExcludeFromTimeTracking:   wt.ExcludeFromTimeTracking,
	}

	if !wt.EndsWith.IsZero() {
		wtpb.EndsWith = wt.EndsWith.Local().Format("2006-01-02")
	}

	return wtpb
}
