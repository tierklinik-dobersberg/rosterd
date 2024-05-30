package roster

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bufbuild/connect-go"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/rosterd/internal/ical"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"github.com/tierklinik-dobersberg/rosterd/templates"
	"go.mongodb.org/mongo-driver/mongo"
)

func (svc *RosterService) ExportRoster(ctx context.Context, req *connect.Request[rosterv1.ExportRosterRequest]) (*connect.Response[rosterv1.ExportRosterResponse], error) {
	// Load the roster from the datastore
	roster, err := svc.Datastore.DutyRosterByID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no roster with id %q found", req.Msg.Id))
		}

		return nil, err
	}

	// get a list of work-shift definitions and index them by key
	workShifts, err := svc.Datastore.ListWorkShifts(ctx)
	if err != nil {
		// TODO(ppacher): better error here?
		return nil, err
	}
	wslm := data.IndexSlice(workShifts, func(shift structs.WorkShift) string { return shift.ID.Hex() })

	// fetch all users and create a lookup map
	allUsers, err := svc.FetchAllUserProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	uslm := data.IndexSlice(allUsers, func(p *idmv1.Profile) string { return p.User.Id })

	// finally, create the export based on the requested type
	switch req.Msg.Type {
	case rosterv1.ExportRosterType_EXPORT_ROSTER_TYPE_ICAL:
		cal := new(ical.Calendar)

		for _, shift := range roster.Shifts {
			def := wslm[shift.WorkShiftID.Hex()]
			if len(req.Msg.IncludeShiftTags) > 0 {

				if !data.ElemInBothSlices(def.Tags, req.Msg.IncludeShiftTags) {
					continue
				}
			}

			users := make([]*idmv1.Profile, len(shift.AssignedUserIds))
			for idx, id := range shift.AssignedUserIds {
				users[idx] = uslm[id]
			}

			cal.Events = append(cal.Events, ical.Event{
				From:  shift.From,
				To:    shift.To,
				Name:  def.Name,
				Users: users,
			})
		}

		export := cal.ToICS(roster.FromTime())

		return connect.NewResponse(&rosterv1.ExportRosterResponse{
			ContentType: "text/calendar",
			FileName:    fmt.Sprintf("%s_%s.ical", roster.From, roster.To),
			Payload:     []byte(export),
		}), nil

	case rosterv1.ExportRosterType_EXPORT_ROSTER_TYPE_PDF,
		rosterv1.ExportRosterType_EXPORT_ROSTER_TYPE_HTML:

		rosterContext := templates.RosterContext{}

		for iter := roster.FromTime(); iter.Before(roster.ToTime().AddDate(0, 0, 1)); iter = iter.AddDate(0, 0, 1) {
			day := templates.RosterDay{
				DayTitle: iter.Format("02.01"),
			}

			for _, shift := range roster.Shifts {
				def := wslm[shift.WorkShiftID.Hex()]

				if shift.From.Format("2006-01-02") == iter.Format("2006-01-02") {
					users := make([]templates.RosterUser, len(shift.AssignedUserIds))
					for idx, id := range shift.AssignedUserIds {
						p := uslm[id]
						users[idx] = templates.RosterUser{
							Name: p.User.Username,
						}
					}

					day.Shifts = append(day.Shifts, templates.RosterShift{
						ShiftName: def.Name,
						Users:     users,
					})
				}
			}

			rosterContext.Days = append(rosterContext.Days, day)

		}

		buf, err := templates.RenderRosterTemplate(ctx, rosterContext)
		if err != nil {
			return nil, err
		}

		blob, err := io.ReadAll(buf)
		if err != nil {
			return nil, err
		}

		if req.Msg.Type == rosterv1.ExportRosterType_EXPORT_ROSTER_TYPE_HTML {
			return connect.NewResponse(&rosterv1.ExportRosterResponse{
				ContentType: "text/html",
				FileName:    fmt.Sprintf("%s_%s.html", roster.From, roster.To),
				Payload:     blob,
			}), nil
		}

		// PDF
		pdf, err := svc.Providers.RenderHTML(ctx, string(blob))
		if err != nil {
			return nil, fmt.Errorf("failed to render HTML: %w", err)
		}
		defer pdf.Close()

		content, err := io.ReadAll(pdf)
		if err != nil {
			return nil, fmt.Errorf("failed to receive PDF: %w", err)
		}

		return connect.NewResponse(&rosterv1.ExportRosterResponse{
			ContentType: "application/pdf",
			FileName:    fmt.Sprintf("%s_%s.pdf", roster.From, roster.To),
			Payload:     content,
		}), nil

	}

	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown export type %q", req.Msg.Type.String()))
}
