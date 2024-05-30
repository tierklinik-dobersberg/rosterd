package workshift

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/mennanov/fmutils"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/rosterd/internal/config"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	rosterv1connect.UnimplementedWorkShiftServiceHandler

	*config.Providers
}

func New(p *config.Providers) *Service {
	return &Service{
		Providers: p,
	}
}

func (svc *Service) ListWorkShifts(ctx context.Context, req *connect.Request[rosterv1.ListWorkShiftsRequest]) (*connect.Response[rosterv1.ListWorkShiftsResponse], error) {
	shifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.ListWorkShiftsResponse{
		WorkShifts: make([]*rosterv1.WorkShift, 0, len(shifts)),
	}

	for _, shift := range shifts {
		// TODO(ppacher): allow the user to query deleted work-shifts as well.
		if shift.Deleted {
			continue
		}

		response.WorkShifts = append(response.WorkShifts, shift.ToProto())
	}

	return connect.NewResponse(response), nil
}

func (svc *Service) CreateWorkShift(ctx context.Context, req *connect.Request[rosterv1.CreateWorkShiftRequest]) (*connect.Response[rosterv1.CreateWorkShiftResponse], error) {
	shift := structs.WorkShift{
		Duration:           structs.JSDuration(req.Msg.Duration.AsDuration()),
		ShortName:          req.Msg.DisplayName,
		Name:               req.Msg.Name,
		OnHoliday:          req.Msg.OnHoliday,
		EligibleRoles:      req.Msg.EligibleRoleIds,
		RequiredStaffCount: int(req.Msg.RequiredStaffCount),
		Color:              req.Msg.Color,
		Description:        req.Msg.Description,
		Order:              int(req.Msg.Order),
		Tags:               req.Msg.Tags,
	}

	shift.From.FromProto(req.Msg.From)

	// verify that all specified roles actually exist.
	for _, roleId := range shift.EligibleRoles {
		_, err := svc.Roles.GetRole(ctx, connect.NewRequest(&idmv1.GetRoleRequest{
			Search: &idmv1.GetRoleRequest_Id{
				Id: roleId,
			},
		}))

		if err != nil {
			return nil, fmt.Errorf("failed to fetch role with id %q: %w", roleId, err)
		}
	}

	if req.Msg.TimeWorth != nil && req.Msg.TimeWorth.IsValid() {
		worth := int(math.Floor(float64(req.Msg.TimeWorth.AsDuration()) / float64(time.Minute)))
		shift.MinutesWorth = &worth
	}

	shift.Days = make([]time.Weekday, len(req.Msg.Days))
	for idx, day := range req.Msg.Days {
		shift.Days[idx] = time.Weekday(day)
	}

	if err := svc.Datastore.SaveWorkShift(ctx, &shift); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.CreateWorkShiftResponse{
		WorkShift: shift.ToProto(),
	}), nil
}

func (svc *Service) UpdateWorkShift(ctx context.Context, req *connect.Request[rosterv1.UpdateWorkShiftRequest]) (*connect.Response[rosterv1.UpdateWorkShiftResponse], error) {
	// load all workshifts
	// TODO(ppacher): add a method to get work-shift by ID
	shifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		return nil, err
	}

	// find the shift that we want to update.
	var shift structs.WorkShift
	for _, s := range shifts {
		if s.ID.Hex() == req.Msg.Id {
			shift = s

			break
		}
	}

	// handle shift-not-found
	if shift.ID.IsZero() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find shift with id %q", req.Msg.Id))
	}

	paths := req.Msg.GetWriteMask().GetPaths()
	if len(paths) > 0 {
		fmutils.Filter(req.Msg.Update, req.Msg.WriteMask.Paths)
	} else {
		paths = []string{
			"from",
			"duration",
			"days",
			"name",
			"display_name",
			"on_holiday",
			"eligible_role_ids",
			"time_worth",
			"required_staff_count",
			"color",
			"description",
			"order",
			"tags",
		}
	}

	for _, p := range paths {
		switch p {
		case "from":
			shift.From.FromProto(req.Msg.Update.From)
		case "duration":
			shift.Duration = structs.JSDuration(req.Msg.Update.Duration.AsDuration())
		case "days":
			shift.Days = make([]time.Weekday, len(req.Msg.Update.Days))
			for idx, d := range req.Msg.Update.Days {
				shift.Days[idx] = time.Weekday(d)
			}
		case "name":
			shift.Name = req.Msg.Update.Name
		case "display_name":
			shift.ShortName = req.Msg.Update.DisplayName
		case "on_holiday":
			shift.OnHoliday = req.Msg.Update.OnHoliday
		case "eligible_role_ids":
			shift.EligibleRoles = req.Msg.Update.EligibleRoleIds
		case "time_worth":
			worth := int(math.Floor(float64(req.Msg.Update.TimeWorth.AsDuration()) / float64(time.Minute)))
			shift.MinutesWorth = &worth
		case "required_staff_count":
			shift.RequiredStaffCount = int(req.Msg.Update.RequiredStaffCount)
		case "color":
			shift.Color = req.Msg.Update.Color
		case "description":
			shift.Description = req.Msg.Update.Description
		case "order":
			shift.Order = int(req.Msg.Update.Order)
		case "tags":
			shift.Tags = req.Msg.Update.Tags

		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported path %q in write_mask", p))
		}
	}

	if !req.Msg.UpdateInPlace {
		if err := svc.Datastore.DeleteWorkShift(ctx, shift.ID.Hex()); err != nil {
			return nil, fmt.Errorf("failed to delete work-shift: %w", err)
		}

		shift.ID = primitive.ObjectID{}
	}

	if err := svc.Datastore.SaveWorkShift(ctx, &shift); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.UpdateWorkShiftResponse{
		WorkShift: shift.ToProto(),
	}), nil
}

func (svc *Service) DeleteWorkShift(ctx context.Context, req *connect.Request[rosterv1.DeleteWorkShiftRequest]) (*connect.Response[rosterv1.DeleteWorkShiftResponse], error) {
	if err := svc.Datastore.DeleteWorkShift(ctx, req.Msg.Id); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.DeleteWorkShiftResponse{}), nil
}
