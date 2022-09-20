package generator

import (
	"math/rand"
	"time"

	"github.com/ccssmnn/hego"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type ShiftAssignment struct {
	Date    string
	ShiftID string
}

type GeneratorState struct {
	schedule         map[string]map[string]structs.RosterShiftWithStaffList
	staffAssignments map[string][]ShiftAssignment
	Users            []structs.User
	getObjective     func(structs.Roster) int
	year             int
	month            time.Month
}

type TabuState struct {
	GeneratorState
}

func NewGeneratorState(year int, month time.Month, requiredShifts map[string][]structs.RosterShiftWithStaffList, users []structs.User, getObjective func(structs.Roster) int) *GeneratorState {
	state := &GeneratorState{
		schedule:         make(map[string]map[string]structs.RosterShiftWithStaffList),
		staffAssignments: make(map[string][]ShiftAssignment),
		Users:            users,
		getObjective:     getObjective,
		year:             year,
		month:            month,
	}

	for _, shifts := range requiredShifts {
		for _, shift := range shifts {
			key := shift.From.Format("2006-01-02")
			if state.schedule[key] == nil {
				state.schedule[key] = make(map[string]structs.RosterShiftWithStaffList)
			}
			state.schedule[key][shift.ShiftID.Hex()] = shift

			// randomly assign staff for this shift
			skip := map[int]struct{}{}
			for i := 0; i < shift.RequiredStaffCount; i++ {
				var idx int
				for {
					idx = rand.Intn(len(shift.EligibleStaff))
					if _, ok := skip[idx]; ok {
						continue
					}
					skip[idx] = struct{}{}

					break
				}

				staff := shift.EligibleStaff[idx]
				state.staffAssignments[staff] = append(state.staffAssignments[staff], ShiftAssignment{
					Date:    shift.From.Format("2006-01-02"),
					ShiftID: shift.ShiftID.Hex(),
				})
			}
		}
	}

	return state
}

func NewTabuState(state GeneratorState) *TabuState {
	return &TabuState{GeneratorState: state}
}

func shiftInSlice(s ShiftAssignment, slice []ShiftAssignment) bool {
	for i := range slice {
		if s.Date == slice[i].Date && s.ShiftID == slice[i].ShiftID {
			return true
		}
	}

	return false
}

func (state *GeneratorState) clone() *GeneratorState {
	newState := &GeneratorState{
		schedule:         state.schedule,
		staffAssignments: make(map[string][]ShiftAssignment),
		Users:            state.Users,
		year:             state.year,
		month:            state.month,
		getObjective:     state.getObjective,
	}

	for user, shifts := range state.staffAssignments {
		newState.staffAssignments[user] = make([]ShiftAssignment, len(shifts))
		copy(newState.staffAssignments[user], state.staffAssignments[user])
	}

	return newState
}

func (state *TabuState) Equal(other hego.TabuState) bool {
	o := other.(*TabuState)

	for _, user := range state.Users {
		if len(state.staffAssignments[user.Name]) != len(o.staffAssignments[user.Name]) {
			return false
		}

		for _, shift := range state.staffAssignments[user.Name] {
			if !shiftInSlice(shift, o.staffAssignments[user.Name]) {
				return false
			}
		}
	}

	return true
}

func (state *GeneratorState) swapShift() *GeneratorState {
	var (
		ua     string
		ub     string
		sha    int
		shb    int
		shiftA ShiftAssignment
		shiftB ShiftAssignment
	)

	for {
		na, nb := rand.Intn(len(state.Users)), rand.Intn(len(state.Users))

		if na == nb {
			continue
		}

		// the the randomly choosen user names
		ua, ub = state.Users[na].Name, state.Users[nb].Name

		// get some random shift index for both users
		sha = rand.Intn(len(state.staffAssignments[ua]))
		shb = rand.Intn(len(state.staffAssignments[ub]))

		shiftA = state.staffAssignments[ua][sha]
		shiftB = state.staffAssignments[ub][shb]

		// make sure we're actually allowed to do the swap
		eligibleShiftA := state.schedule[shiftA.Date][shiftA.ShiftID].EligibleStaff
		eligibleShiftB := state.schedule[shiftB.Date][shiftB.ShiftID].EligibleStaff

		if !stringInSlice(ua, eligibleShiftB) || !stringInSlice(ub, eligibleShiftA) {
			continue
		}

		break
	}

	newState := state.clone()
	newState.staffAssignments[ua][sha] = shiftB
	newState.staffAssignments[ub][shb] = shiftA

	return newState
}

func stringInSlice(s string, slice []string) bool {
	for idx := range slice {
		if s == slice[idx] {
			return true
		}
	}

	return false
}

func (state *GeneratorState) transferShift() *GeneratorState {
	var (
		sourceStaff      string
		destinationStaff string
		lenSource        int
		sh               int
	)
	for {
		// get some random users
		var source, destination int
		source, destination = rand.Intn(len(state.Users)), rand.Intn(len(state.Users))

		sourceStaff, destinationStaff = state.Users[source].Name, state.Users[destination].Name

		if len(state.staffAssignments[destinationStaff]) > len(state.staffAssignments[sourceStaff]) {
			sourceStaff, destinationStaff = destinationStaff, sourceStaff
		}

		lenSource = len(state.staffAssignments[sourceStaff])
		sh = rand.Intn(lenSource)

		// make sure that destinationStaff is actually eligible for the shift we want
		// to transfer from sourceStaff
		shiftSource := state.staffAssignments[sourceStaff][sh]
		if !stringInSlice(destinationStaff, state.schedule[shiftSource.Date][shiftSource.ShiftID].EligibleStaff) {
			continue
		}

		break
	}

	newState := state.clone()
	newState.staffAssignments[destinationStaff] = append(newState.staffAssignments[destinationStaff], newState.staffAssignments[sourceStaff][sh])
	newState.staffAssignments[sourceStaff][sh] = newState.staffAssignments[sourceStaff][lenSource-1]
	newState.staffAssignments[sourceStaff] = newState.staffAssignments[sourceStaff][:lenSource-1]

	return newState
}

func (state *TabuState) Neighbor() hego.TabuState {
	if rand.Float64() < 0.5 {
		return &TabuState{
			GeneratorState: *state.swapShift(),
		}
	}
	return &TabuState{
		GeneratorState: *state.transferShift(),
	}
}

func (state *GeneratorState) ToRoster() structs.Roster {
	r := structs.Roster{
		Month: state.month,
		Year:  state.year,
	}

	shiftLookupMap := make(map[string]*structs.RosterShiftWithStaffList)
	for dateKey, shiftMap := range state.schedule {
		for shiftKey := range shiftMap {
			shift := shiftMap[shiftKey]
			shiftLookupMap[dateKey+"-"+shiftKey] = &shift
		}
	}

	for user, shifts := range state.staffAssignments {
		for _, shift := range shifts {
			key := shift.Date + "-" + shift.ShiftID
			shiftLookupMap[key].Staff = append(shiftLookupMap[key].Staff, user)
		}
	}

	for _, shift := range shiftLookupMap {
		r.Shifts = append(r.Shifts, shift.RosterShift)
	}

	return r
}

func (state *GeneratorState) Objective() float64 {
	r := state.ToRoster()

	return float64(state.getObjective(r))
}
