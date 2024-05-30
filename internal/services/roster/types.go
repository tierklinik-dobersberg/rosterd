package roster

import (
	"context"
	"errors"

	"github.com/bufbuild/connect-go"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/mongo"
)

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
