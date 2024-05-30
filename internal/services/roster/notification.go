package roster

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/mitchellh/mapstructure"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/ical"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/structpb"
)

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

	isPreview := !roster.Approved

	deliveries, err := svc.sendRosterNotification(ctx, remoteUser.ID, roster, isPreview, req.Msg.SendNotificationToUsers)
	if err != nil {
		return nil, err
	}

	res := connect.NewResponse(&rosterv1.SendRosterPreviewResponse{
		Delivery: deliveries,
	})

	return res, nil
}

func (svc *RosterService) sendRosterNotification(ctx context.Context, senderId string, roster structs.DutyRoster, isPreview bool, receipients []string) ([]*idmv1.DeliveryNotification, error) {
	type Shift struct {
		Name string
		From string
		To   string
	}

	var (
		perUserShifts = make(map[string]map[string][]Shift)
		targetUsers   = make(map[string]*idmv1.Profile)
	)

	calendar := new(ical.Calendar)

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

		event := ical.Event{
			Name: shiftName,
			From: shift.From,
			To:   shift.To,
		}

		for _, usrId := range shift.AssignedUserIds {
			targetUsers[usrId] = userLm[usrId]

			event.Users = append(event.Users, userLm[usrId])

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

		calendar.Events = append(calendar.Events, event)
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

	// filter out any user that is not part of receipienets
	if len(receipients) > 0 {
		for key := range targetUsers {
			if !slices.Contains(receipients, key) {
				delete(targetUsers, key)
			}
		}
	}

	userIds := maps.Keys(targetUsers)

	workTime, err := svc.analyzeWorkTime(ctx, userIds, roster.From, roster.To, true)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze work time: %w", err)
	}

	userWorkTimes, err := svc.Datastore.GetCurrentWorkTimes(ctx, roster.ToTime())
	if err != nil {
		return nil, fmt.Errorf("failed to get user work times: %w", err)
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
			"Dates":                   shiftMaps,
			"ExpectedTime":            int64(userWorkTime.ExpectedTime.AsDuration().Seconds()),
			"PlannedTime":             int64(userWorkTime.PlannedTime.AsDuration().Seconds()),
			"Overtime":                int64(userWorkTime.Overtime.AsDuration().Seconds()),
			"Preview":                 isPreview,
			"RosterDate":              roster.FromTime().Format("2006/01"),
			"RosterURL":               fmt.Sprintf(svc.Config.PreviewRosterURL, roster.ID.Hex()),
			"ExcludeFromTimeTracking": userWorkTimes[userId].ExcludeFromTimeTracking,
			"From":                    roster.From,
			"To":                      roster.To,
			"Diff":                    diffMaps,
			"Superseded":              isSuperseded,
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

	log.L(ctx).WithField("targetUsers", userIds).Infof("sending roster notification")

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
