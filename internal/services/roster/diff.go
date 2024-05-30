package roster

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"golang.org/x/exp/slices"
)

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
