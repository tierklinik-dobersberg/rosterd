package constraint

import (
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type EvalContext struct {
	structs.RosterShift

	Roster *structs.Roster
	Next   structs.RosterShift
	Prev   structs.RosterShift

	Staff string
	Day   string
}

func (ctx EvalContext) HasOnSameDay(shiftName string) bool {
	if ctx.Roster == nil {
		return false
	}

	yearDay := ctx.From.YearDay()
	for _, shift := range ctx.Roster.Shifts {
		if shift.From.YearDay() == yearDay && shift.Name == shiftName {
			for _, s := range shift.Staff {
				if ctx.Staff == s {
					return true
				}
			}
		}
	}

	return false
}

func (ctx EvalContext) CountShiftTypes(shiftName string) int {
	if ctx.Roster == nil {
		return 0
	}

	count := 0
	for _, shift := range ctx.Roster.Shifts {
		if shift.Name != shiftName {
			continue
		}

		for _, s := range shift.Staff {
			if ctx.Staff == s {
				count++
				break
			}
		}
	}

	return count
}

func (ctx EvalContext) NumberOfWeeks() int {
	if ctx.Roster == nil {
		return 0
	}

	m := make(map[int]struct{})
	for _, shift := range ctx.Roster.Shifts {
		_, week := shift.From.ISOWeek()
		m[week] = struct{}{}
	}

	return len(m)
}

func (ctx *EvalContext) init() {
	ctx.Next = ctx.next()
	ctx.Prev = ctx.previous()
}

func (ctx *EvalContext) previous() structs.RosterShift {
	if ctx.Roster == nil {
		return structs.RosterShift{}
	}

	var (
		lastShift structs.RosterShift
		found     = false
	)

	for idx, shift := range ctx.Roster.Shifts {
		if shift.ShiftID == ctx.ShiftID && shift.From.Equal(ctx.From) && shift.To.Equal(ctx.To) {
			break
		}

		for _, staff := range shift.Staff {
			if staff == ctx.Staff {
				found = false
				lastShift = ctx.Roster.Shifts[idx]
				break
			}
		}
	}

	if !found {
		return structs.RosterShift{}
	}

	return lastShift
}

func (ctx *EvalContext) next() structs.RosterShift {
	if ctx.Roster == nil {
		return structs.RosterShift{}
	}

	var idx int
	for idx = range ctx.Roster.Shifts {
		shift := ctx.Roster.Shifts[idx]
		if shift.ShiftID == ctx.ShiftID && shift.From.Equal(ctx.From) && shift.To.Equal(ctx.To) {
			break
		}
	}

	idx++
	if idx >= len(ctx.Roster.Shifts) {
		return structs.RosterShift{}
	}

	for _, staff := range ctx.Roster.Shifts[idx].Staff {
		if staff == ctx.Staff {
			return ctx.Roster.Shifts[idx]
		}
	}

	return structs.RosterShift{}
}
