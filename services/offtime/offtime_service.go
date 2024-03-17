package offtime

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/mennanov/fmutils"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Database interface {
	database.OffTimeDatabase
	database.WorkTimeDatabase
}

type Service struct {
	rosterv1connect.UnimplementedOffTimeServiceHandler
	*config.Providers
}

func New(providers *config.Providers) *Service {
	return &Service{
		Providers: providers,
	}
}

func (svc *Service) GetOffTimeEntry(ctx context.Context, req *connect.Request[rosterv1.GetOffTimeEntryRequest]) (*connect.Response[rosterv1.GetOffTimeEntryResponse], error) {
	entries, err := svc.Datastore.GetOffTimeRequest(ctx, req.Msg.Ids...)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.GetOffTimeEntryResponse{
		Entry: make([]*rosterv1.OffTimeEntry, len(entries)),
	}

	for idx, e := range entries {
		response.Entry[idx] = e.ToProto()
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) CreateOffTimeRequest(ctx context.Context, req *connect.Request[rosterv1.CreateOffTimeRequestRequest]) (*connect.Response[rosterv1.CreateOffTimeRequestResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// figure out for which user we want to create the offtime request
	if req.Msg.RequestorId == "" || !remoteUser.Admin {
		req.Msg.RequestorId = remoteUser.ID
	}

	if req.Msg.RequestorId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to determine target user"))
	}

	// verify the user actually exists.
	if err := svc.verifyUsersExists(ctx, req.Msg.RequestorId); err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	from := req.Msg.From.AsTime()
	to := req.Msg.To.AsTime()

	// actually create the off-time entry and store it in the database.
	entry := structs.OffTimeEntry{
		From:        from,
		To:          to,
		Description: req.Msg.Description,
		RequestorId: req.Msg.RequestorId,
		CreatedAt:   time.Now(),
		CreatorId:   remoteUser.ID,
		RequestType: requestTypeFromProto(req.Msg.RequestType),
	}

	if err := svc.Datastore.CreateOffTimeRequest(ctx, &entry); err != nil {
		return nil, err
	}

	go func() {
		managerUsers, err := svc.Providers.Users.ListUsers(context.Background(), connect.NewRequest(&idmv1.ListUsersRequest{
			FilterByRoles: []string{svc.Config.RosterManagerRoleID},
			FieldMask: &fieldmaskpb.FieldMask{
				Paths: []string{"users.user.id"},
			},
		}))

		userIds := maps.Keys(data.IndexSlice(managerUsers.Msg.Users, func(p *idmv1.Profile) string {
			return p.User.Id
		}))

		if err != nil {
			log.L(context.Background()).Errorf("failed to get roster_manager users: %s", err)
		}

		svc.Providers.Notify.SendNotification(context.Background(), connect.NewRequest(&idmv1.SendNotificationRequest{
			Message: &idmv1.SendNotificationRequest_Webpush{
				Webpush: &idmv1.WebPushNotification{
					Kind: &idmv1.WebPushNotification_Notification{
						Notification: &idmv1.ServiceWorkerNotification{
							Title:               "Tierklinik-Dobersberg",
							Body:                "{{ .Sender | displayName }} hat einen Urlaubsantrag erstellt",
							DefaultOperation:    idmv1.Operation_OPERATION_OPEN_WINDOW,
							DefaultOperationUrl: svc.Config.PublicURL + "/offtimes",
						},
					},
				},
			},
			SenderUserId: remoteUser.ID,
			TargetUsers:  userIds,
		}))
	}()

	return connect.NewResponse(&rosterv1.CreateOffTimeRequestResponse{
		Entry: entry.ToProto(),
	}), nil
}

func (svc *Service) UpdateOffTimeRequest(ctx context.Context, req *connect.Request[rosterv1.UpdateOffTimeRequestRequest]) (*connect.Response[rosterv1.UpdateOffTimeRequestResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	entries, err := svc.Datastore.GetOffTimeRequest(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("off-time request with id %q not found", req.Msg.Id))
	}

	if len(entries) > 1 {
		return nil, fmt.Errorf("internal: multiple off-time requests with the same ID")
	}

	entry := entries[0]

	if entry.Approval != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("off-time request has already been approved or rejected and cannot be modified anymore"))
	}

	if !remoteUser.Admin && entry.RequestorId != remoteUser.ID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you are not allowed to update this off-time request"))
	}

	paths := []string{
		"from",
		"to",
		"requestor_id",
		"description",
		"request_type",
	}

	if p := req.Msg.FieldMask.GetPaths(); len(p) > 0 {
		paths = p
	}

	for _, p := range paths {
		switch p {
		case "from":
			if !req.Msg.From.IsValid() {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("from field is invalid"))
			}

			entry.From = req.Msg.From.AsTime()

		case "to":
			if !req.Msg.To.IsValid() {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("to field is invalid"))
			}

			entry.To = req.Msg.To.AsTime()

		case "requestor_id":
			if req.Msg.RequestorId != "" {
				if !remoteUser.Admin {
					if req.Msg.RequestorId != entry.RequestorId {
						return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you are not allowed to change the requestor"))
					}
				}

				if err := svc.verifyUsersExists(ctx, req.Msg.RequestorId); err != nil {
					return nil, err
				}

				entry.RequestorId = req.Msg.RequestorId
			} else {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("requestor must be set"))
			}

		case "description":
			entry.Description = req.Msg.Description
		case "request_type":
			entry.RequestType = requestTypeFromProto(req.Msg.RequestType)
		}
	}

	if err := svc.Datastore.UpdateOffTimeRequest(ctx, &entry); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.UpdateOffTimeRequestResponse{
		Entry: entry.ToProto(),
	}), nil
}

func (svc *Service) DeleteOffTimeRequest(ctx context.Context, req *connect.Request[rosterv1.DeleteOffTimeRequestRequest]) (*connect.Response[rosterv1.DeleteOffTimeRequestResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	models, err := svc.Datastore.GetOffTimeRequest(ctx, req.Msg.Id...)
	if err != nil {
		return nil, err
	}

	lm := data.IndexSlice(models, func(m structs.OffTimeEntry) string { return m.ID.Hex() })

	currentUserId := remoteUser.ID

	for _, id := range req.Msg.Id {
		model, ok := lm[id]
		if !ok {
			return nil, fmt.Errorf("id: %s off-time-entry not found", id)
		}

		if model.Approval != nil && !remoteUser.Admin {
			return nil, fmt.Errorf("id: %s off-time-entry has already been approved/rejected", id)
		}

		now := time.Now()
		if (now.After(model.To) || now.After(model.From)) && !remoteUser.Admin {
			return nil, fmt.Errorf("id: %s off-time entry is already in the past", id)
		}

		if !remoteUser.Admin && model.RequestorId != currentUserId {
			return nil, fmt.Errorf("id: %s: you are not allowed to delete this entry", id)
		}
	}

	if err := svc.Datastore.DeleteOffTimeRequest(ctx, req.Msg.Id...); err != nil {
		return nil, err
	}

	return connect.NewResponse(new(rosterv1.DeleteOffTimeRequestResponse)), nil
}

func (svc *Service) FindOffTimeRequests(ctx context.Context, req *connect.Request[rosterv1.FindOffTimeRequestsRequest]) (*connect.Response[rosterv1.FindOffTimeRequestsResponse], error) {
	var approved *bool

	if req.Msg.Approved != nil {
		approved = &req.Msg.Approved.Value
	}

	var from time.Time
	var to time.Time

	if req.Msg.From.IsValid() {
		from = req.Msg.From.AsTime()
	}

	if req.Msg.To.IsValid() {
		to = req.Msg.To.AsTime()
	}

	res, err := svc.Datastore.FindOffTimeRequests(
		ctx,
		from,
		to,
		approved,
		req.Msg.UserIds,
	)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.FindOffTimeRequestsResponse{
		Results: make([]*rosterv1.OffTimeEntry, len(res)),
	}

	for idx, r := range res {
		response.Results[idx] = r.ToProto()
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) ApproveOrReject(ctx context.Context, req *connect.Request[rosterv1.ApproveOrRejectRequest]) (*connect.Response[rosterv1.ApproveOrRejectResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	models, err := svc.Datastore.GetOffTimeRequest(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find request"))
	}

	approval := structs.Approval{
		Approved:   req.Msg.Type == rosterv1.ApprovalRequestType_APPROVAL_REQUEST_TYPE_APPROVED,
		ApprovedAt: time.Now(),
		ApproverID: remoteUser.ID,
		Comment:    req.Msg.Comment,
	}

	if err := svc.Datastore.ApproveOffTimeRequest(ctx, req.Msg.Id, &approval); err != nil {
		return nil, err
	}

	models, err = svc.Datastore.GetOffTimeRequest(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("failed to find approved request")
	}

	if err := svc.sendApprovalNotice(ctx, remoteUser.ID, models[0]); err != nil {
		log.L(ctx).Errorf("failed to send approval notice to %q: %s", remoteUser.ID, err)
	}

	return connect.NewResponse(&rosterv1.ApproveOrRejectResponse{
		Entry: models[0].ToProto(),
	}), nil
}

func (svc *Service) AddOffTimeCosts(ctx context.Context, req *connect.Request[rosterv1.AddOffTimeCostsRequest]) (*connect.Response[rosterv1.AddOffTimeCostsResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	for _, costs := range req.Msg.AddCosts {
		model := structs.OffTimeCosts{
			CreatedAt:  time.Now(),
			CreatorId:  remoteUser.ID,
			Costs:      costs.Costs.AsDuration(),
			IsVacation: costs.IsVacation,
			UserID:     costs.UserId,
			Comment:    costs.Comment,
		}

		if costs.Date.IsValid() {
			model.Date = costs.Date.AsTime()
		}

		// make sure the user actually exists
		if err := svc.verifyUsersExists(ctx, costs.UserId); err != nil {
			return nil, err
		}

		if costs.OfftimeId != "" {
			var err error

			model.OfftimeID, err = primitive.ObjectIDFromHex(costs.OfftimeId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid offtime_id: %w", err))
			}

			// make sure the off-time entry actually exists.
			entries, err := svc.Datastore.GetOffTimeRequest(ctx, costs.OfftimeId)
			if err != nil {
				return nil, fmt.Errorf("failed to get offtime-request %s: %w", costs.OfftimeId, err)
			}
			if len(entries) == 0 {
				return nil, fmt.Errorf("failed to get offtime-request %s", costs.OfftimeId)
			}

			if entries[0].RequestorId != model.UserID {
				return nil, fmt.Errorf("referenced offtime-request does not belong to the same user")
			}

			if model.Date.IsZero() {
				costs.Date = timestamppb.New(entries[0].From)
			}
		}

		if costs.RosterId != "" {
			var err error

			model.RosterID, err = primitive.ObjectIDFromHex(costs.RosterId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid roster_id: %w", err))
			}

			// make sure the roster actually exists.
			_, err = svc.Datastore.DutyRosterByID(ctx, costs.RosterId)
			if err != nil {
				return nil, fmt.Errorf("failed to get roster %s: %w", costs.RosterId, err)
			}
		}

		if model.Date.IsZero() {
			model.Date = time.Now()
		}

		if model.Costs == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid costs"))
		}

		if err := svc.Datastore.AddOffTimeCost(ctx, &model); err != nil {
			return nil, err
		}
	}

	return connect.NewResponse(new(rosterv1.AddOffTimeCostsResponse)), nil
}

func (svc *Service) GetOffTimeCosts(ctx context.Context, req *connect.Request[rosterv1.GetOffTimeCostsRequest]) (*connect.Response[rosterv1.GetOffTimeCostsResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	// determine for which Users we want to load costs
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
		// if unspecified, load all users for admin and only the owner if not
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

	log.L(ctx).Infof("loading off-time costs for users: %v", userIds)

	costs, err := svc.Datastore.GetOffTimeCosts(ctx, userIds...)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.GetOffTimeCostsResponse{}

	m := make(map[string][]structs.OffTimeCosts)
	for idx := range costs {
		cost := costs[idx]

		m[cost.UserID] = append(m[cost.UserID], cost)

		log.L(ctx).Infof("adding new offtime cost to user %q", cost.UserID)
	}

	for user, costs := range m {
		res := &rosterv1.UserOffTimeCosts{
			UserId: user,
			Costs:  make([]*rosterv1.OffTimeCosts, len(costs)),
		}

		var (
			sumVacation time.Duration
			sumTimeOff  time.Duration
		)

		for idx, c := range costs {
			res.Costs[idx] = &rosterv1.OffTimeCosts{
				Id:         c.ID.Hex(),
				UserId:     c.UserID,
				CreatedAt:  timestamppb.New(c.CreatedAt),
				CreatorId:  c.CreatorId,
				Costs:      durationpb.New(c.Costs),
				IsVacation: c.IsVacation,
				Date:       timestamppb.New(c.Date),
				Comment:    c.Comment,
			}

			if !c.OfftimeID.IsZero() {
				res.Costs[idx].OfftimeId = c.OfftimeID.Hex()
			}

			if !c.RosterID.IsZero() {
				res.Costs[idx].RosterId = c.RosterID.Hex()
			}

			if c.IsVacation {
				sumVacation += c.Costs
			} else {
				sumTimeOff += c.Costs
			}
		}

		log.L(ctx).Infof("prepared off-time cost summary for user %q with %d results and %s vacation / %s timeoff", user, len(res.Costs), sumVacation, sumTimeOff)

		res.Summary = &rosterv1.OffTimeCostSummary{
			Vacation: durationpb.New(sumVacation),
			TimeOff:  durationpb.New(sumTimeOff),
		}

		response.Results = append(response.Results, res)
	}

	if paths := req.Msg.GetReadMask().GetPaths(); len(paths) > 0 {
		log.L(ctx).Infof("filtering response based on get-paths: %v", paths)
		fmutils.Filter(response, paths)
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) DeleteOffTimeCosts(ctx context.Context, req *connect.Request[rosterv1.DeleteOffTimeCostsRequest]) (*connect.Response[rosterv1.DeleteOffTimeCostsResponse], error) {
	if err := svc.Datastore.DeleteOffTimeCosts(ctx, req.Msg.Ids...); err != nil {
		return nil, err
	}

	return connect.NewResponse(new(rosterv1.DeleteOffTimeCostsResponse)), nil
}

func (svc *Service) sendApprovalNotice(ctx context.Context, sender string, entry structs.OffTimeEntry) error {
	renderCtx := map[string]any{
		"Approved":    entry.Approval.Approved,
		"Comment":     entry.Approval.Comment,
		"From":        entry.From.Format("2006-01-02"),
		"To":          entry.To.Format("2006-01-02"),
		"Description": entry.Description,
	}

	switch entry.RequestType {
	case structs.RequestTypeVacation:
		renderCtx["Type"] = "vacation"
	case structs.RequestTypeTimeOff:
		fallthrough
	default:
		renderCtx["Type"] = "timeOff"
	}

	ctxPb, err := structpb.NewStruct(renderCtx)
	if err != nil {
		return fmt.Errorf("failed to prepare structpb context: %w", err)
	}

	templateBody, err := fs.ReadFile(svc.Templates, "mails/dist/offtime-notification.html")
	if err != nil {
		return fmt.Errorf("failed to read offtime-notification template: %w", err)
	}

	_, err = svc.Providers.Notify.SendNotification(context.Background(), connect.NewRequest(&idmv1.SendNotificationRequest{
		Message: &idmv1.SendNotificationRequest_Webpush{
			Webpush: &idmv1.WebPushNotification{
				Kind: &idmv1.WebPushNotification_Notification{
					Notification: &idmv1.ServiceWorkerNotification{
						Title:               "Tierklinik-Dobersberg",
						Body:                `Dein Urlaubsantrag von {{ .From }} bis {{ .To }} wurde {{ if .Approved }} genehmigt {{ else }} abgelehnt {{ end }}`,
						DefaultOperation:    idmv1.Operation_OPERATION_OPEN_WINDOW,
						DefaultOperationUrl: svc.Config.PublicURL + "/offtimes",
					},
				},
			},
		},
		SenderUserId: sender,
		PerUserTemplateContext: map[string]*structpb.Struct{
			entry.RequestorId: ctxPb,
		},
		TargetUsers: []string{entry.RequestorId},
	}))
	if err != nil {
		log.L(ctx).Errorf("failed to send web-push notification: %s", err)
	}

	_, err = svc.Notify.SendNotification(ctx, connect.NewRequest(&idmv1.SendNotificationRequest{
		Message: &idmv1.SendNotificationRequest_Email{
			Email: &idmv1.EMailMessage{
				Subject: "Dein Urlaubs/ZA Antrag wurde bearbeitet",
				Body:    string(templateBody),
			},
		},
		TargetUsers: []string{entry.RequestorId},
		PerUserTemplateContext: map[string]*structpb.Struct{
			entry.RequestorId: ctxPb,
		},
		SenderUserId: sender,
	}))

	return err
}

func requestTypeFromProto(rtype rosterv1.OffTimeType) structs.RequestType {
	switch rtype {
	case rosterv1.OffTimeType_OFF_TIME_TYPE_TIME_OFF:
		return structs.RequestTypeTimeOff
	case rosterv1.OffTimeType_OFF_TIME_TYPE_VACATION:
		return structs.RequestTypeVacation
	default:
		return structs.RequestTypeAuto
	}
}

func (svc *Service) verifyUsersExists(ctx context.Context, userId string) error {
	_, err := svc.Users.GetUser(ctx, connect.NewRequest(&idmv1.GetUserRequest{
		Search: &idmv1.GetUserRequest_Id{
			Id: userId,
		},
	}))

	return err
}

func (svc *Service) calculateWorkTimeBetween(ctx context.Context, from, to time.Time) (map[string]time.Duration, map[string]structs.WorkTime, error) {
	res, err := svc.Holidays.NumberOfWorkDays(ctx, connect.NewRequest(&calendarv1.NumberOfWorkDaysRequest{
		From: timestamppb.New(from),
		To:   timestamppb.New(to),
	}))
	if err != nil {
		return nil, nil, err
	}

	currentWorkTimes, err := svc.Datastore.GetCurrentWorkTimes(ctx, from)
	if err != nil {
		return nil, nil, err
	}

	result := make(map[string]time.Duration)

	for user, workTime := range currentWorkTimes {
		timePerWeekday := workTime.TimePerWeek / 5
		result[user] = timePerWeekday * time.Duration(res.Msg.NumberOfWorkDays)
	}

	return result, currentWorkTimes, nil
}

func (svc *Service) calculateMonthlyWorkTime(ctx context.Context, month time.Month, year int) (map[string]time.Duration, error) {
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local)

	// find the number of working-days in the given month
	result, _, err := svc.calculateWorkTimeBetween(ctx, from, to)
	return result, err
}
