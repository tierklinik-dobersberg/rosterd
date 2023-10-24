package roster

import (
	"context"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1/rosterv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/rosterd/config"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ConstraintService struct {
	rosterv1connect.UnimplementedConstraintServiceHandler

	*config.Providers
}

func NewConstraintService(p *config.Providers) *ConstraintService {
	return &ConstraintService{
		Providers: p,
	}
}

func (svc *ConstraintService) CreateConstraint(ctx context.Context, req *connect.Request[rosterv1.CreateConstraintRequest]) (*connect.Response[rosterv1.CreateConstraintResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	model := structs.Constraint{
		Description:   req.Msg.Description,
		Expression:    req.Msg.Expression,
		AppliesToRole: req.Msg.RoleIds,
		AppliesToUser: req.Msg.UserIds,
		Hard:          req.Msg.Hard,
		Penalty:       int(req.Msg.Penalty),
		Deny:          req.Msg.Deny,
		RosterOnly:    req.Msg.RosterOnly,
		CreatedAt:     time.Now(),
		CreatorId:     remoteUser.ID,
		UpdatedAt:     time.Now(),
		LastUpdatedBy: remoteUser.ID,
	}

	if err := svc.validateModel(ctx, &model); err != nil {
		return nil, err
	}

	if err := svc.Datastore.CreateConstraint(ctx, &model); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.CreateConstraintResponse{
		Constraint: model.ToProto(),
	}), nil
}

func (svc *ConstraintService) UpdateConstraint(ctx context.Context, req *connect.Request[rosterv1.UpdateConstraintRequest]) (*connect.Response[rosterv1.UpdateConstraintResponse], error) {
	remoteUser := auth.From(ctx)
	if remoteUser == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	model, err := svc.Datastore.GetConstraintByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	model.UpdatedAt = time.Now()
	model.LastUpdatedBy = remoteUser.ID

	paths := []string{
		"description",
		"expression",
		"role_ids",
		"user_ids",
		"hard",
		"penalty",
		"deny",
		"roster_only",
	}

	if len(req.Msg.WriteMask.GetPaths()) > 0 {
		paths = req.Msg.WriteMask.Paths
	}

	for _, p := range paths {
		switch p {
		case "description":
			model.Description = req.Msg.Description
		case "expression":
			model.Expression = req.Msg.Expression
		case "role_ids":
			model.AppliesToRole = req.Msg.RoleIds
		case "user_ids":
			model.AppliesToUser = req.Msg.UserIds
		case "hard":
			model.Hard = req.Msg.Hard
		case "penalty":
			model.Penalty = int(req.Msg.Penalty)
		case "deny":
			model.Deny = req.Msg.Deny
		case "roster_only":
			model.RosterOnly = req.Msg.RosterOnly
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported write_mask.path: %q", p))
		}
	}

	if err := svc.validateModel(ctx, model); err != nil {
		return nil, err
	}

	if err := svc.Datastore.UpdateConstraint(ctx, model); err != nil {
		return nil, err
	}

	return connect.NewResponse(&rosterv1.UpdateConstraintResponse{
		Constraint: model.ToProto(),
	}), nil
}

func (svc *ConstraintService) DeleteConstraint(ctx context.Context, req *connect.Request[rosterv1.DeleteConstraintRequest]) (*connect.Response[rosterv1.DeleteConstraintResponse], error) {
	if err := svc.Datastore.DeleteConstraint(ctx, req.Msg.Id); err != nil {
		return nil, err
	}

	return connect.NewResponse(new(rosterv1.DeleteConstraintResponse)), nil
}

func (svc *ConstraintService) FindConstraints(ctx context.Context, req *connect.Request[rosterv1.FindConstraintsRequest]) (*connect.Response[rosterv1.FindConstraintsResponse], error) {
	res, err := svc.Datastore.FindConstraints(ctx, req.Msg.UserIds, req.Msg.RoleIds)
	if err != nil {
		return nil, err
	}

	response := &rosterv1.FindConstraintsResponse{
		Results: make([]*rosterv1.Constraint, len(res)),
	}

	for idx, model := range res {
		response.Results[idx] = model.ToProto()
	}

	return connect.NewResponse(response), nil
}

func (svc *ConstraintService) validateModel(ctx context.Context, model *structs.Constraint) error {

	// verify role ids
	if len(model.AppliesToRole) > 0 {
		roleIds, err := svc.fetchRoleIds(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}
		for _, role := range model.AppliesToRole {
			if !slices.Contains(roleIds, role) {
				return connect.NewError(connect.CodeNotFound, fmt.Errorf("role with id %q not found", role))
			}
		}
	}

	// verify user ids
	if len(model.AppliesToUser) > 0 {
		userIds, err := svc.fetchUserIds(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}
		for _, user := range model.AppliesToUser {
			if !slices.Contains(userIds, user) {
				return connect.NewError(connect.CodeNotFound, fmt.Errorf("user with id %q not found", user))
			}
		}
	}

	return nil
}

func (svc *ConstraintService) fetchRoleIds(ctx context.Context) ([]string, error) {
	res, err := svc.Roles.ListRoles(ctx, connect.NewRequest(&idmv1.ListRolesRequest{}))
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(res.Msg.Roles))
	for idx, role := range res.Msg.Roles {
		ids[idx] = role.Id
	}

	return ids, nil
}

func (svc *ConstraintService) fetchUserIds(ctx context.Context) ([]string, error) {
	res, err := svc.Users.ListUsers(ctx, connect.NewRequest(&idmv1.ListUsersRequest{
		FieldMask: &fieldmaskpb.FieldMask{
			Paths: []string{"users.user.id"},
		},
	}))
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(res.Msg.Users))
	for idx, profile := range res.Msg.Users {
		ids[idx] = profile.User.Id
	}

	return ids, nil
}
