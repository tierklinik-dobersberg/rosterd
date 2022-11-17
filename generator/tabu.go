package generator

import (
	"log"
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
	schedule          map[string]map[string]structs.RosterShiftWithStaffList
	staffAssignments  map[string][]ShiftAssignment
	Users             []structs.User
	expectedWorkTimes map[string]time.Duration
	plannedWorkTimes  map[string]time.Duration
	getObjective      func(structs.Roster) int
	year              int
	month             time.Month
}

type TabuState struct {
	GeneratorState
}

func NewGeneratorState(year int, month time.Month, requiredShifts map[string][]structs.RosterShiftWithStaffList, users []structs.User, expectedWorkTimes map[string]time.Duration, getObjective func(structs.Roster) int) *GeneratorState {
	state := &GeneratorState{
		schedule:          make(map[string]map[string]structs.RosterShiftWithStaffList),
		staffAssignments:  make(map[string][]ShiftAssignment),
		expectedWorkTimes: expectedWorkTimes,
		plannedWorkTimes:  make(map[string]time.Duration),
		Users:             users,
		getObjective:      getObjective,
		year:              year,
		month:             month,
	}

	count := 0
	timeRequired := time.Duration(0)
	totalPlanned := time.Duration(0)

	for _, shifts := range requiredShifts {
		for _, shift := range shifts {
			key := shift.From.Format("2006-01-02")
			if state.schedule[key] == nil {
				state.schedule[key] = make(map[string]structs.RosterShiftWithStaffList)
			}
			state.schedule[key][shift.ShiftID.Hex()] = shift

			timeRequired += (time.Duration(shift.MinutesWorth) * time.Minute * time.Duration(shift.RequiredStaffCount))

			// randomly assign staff for this shift
			skip := map[int]struct{}{}
			for i := 0; i < shift.RequiredStaffCount; i++ {
				var (
					idx   int
					staff string
				)

				for {
					idx = rand.Intn(len(shift.EligibleStaff))
					if _, ok := skip[idx]; ok {
						continue
					}
					staff = shift.EligibleStaff[idx]

					if !state.canAddWorkTime(staff, time.Duration(shift.MinutesWorth)*time.Minute) && rand.Float64() > 0.1 {
						continue
					}

					skip[idx] = struct{}{}

					break
				}

				state.plannedWorkTimes[staff] = state.plannedWorkTimes[staff] + (time.Duration(shift.MinutesWorth) * time.Minute)
				totalPlanned += (time.Duration(shift.MinutesWorth) * time.Minute)
				state.staffAssignments[staff] = append(state.staffAssignments[staff], ShiftAssignment{
					Date:    shift.From.Format("2006-01-02"),
					ShiftID: shift.ShiftID.Hex(),
				})
				count++
			}
		}
	}

	log.Printf("assigned %d staff members and planned %s out of required %s", count, totalPlanned.String(), timeRequired.String())
	for _, user := range state.Users {
		log.Printf("%s: expected %s diff-planned: %s", user.Name, state.expectedWorkTimes[user.Name].String(), state.diffWorkTime(user.Name))
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
		schedule:          state.schedule,
		staffAssignments:  make(map[string][]ShiftAssignment),
		Users:             state.Users,
		year:              state.year,
		month:             state.month,
		getObjective:      state.getObjective,
		expectedWorkTimes: state.expectedWorkTimes,
		plannedWorkTimes:  make(map[string]time.Duration),
	}

	for user, shifts := range state.staffAssignments {
		newState.staffAssignments[user] = make([]ShiftAssignment, len(shifts))
		copy(newState.staffAssignments[user], state.staffAssignments[user])
	}

	for user, time := range state.plannedWorkTimes {
		newState.plannedWorkTimes[user] = time
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
		shiftA := state.schedule[shiftA.Date][shiftA.ShiftID]
		shiftB := state.schedule[shiftB.Date][shiftB.ShiftID]

		if !stringInSlice(ua, shiftB.EligibleStaff) || !stringInSlice(ub, shiftA.EligibleStaff) {
			continue
		}

		timeWorthA := time.Minute * time.Duration(shiftA.MinutesWorth)
		timeWorthB := time.Minute * time.Duration(shiftB.MinutesWorth)

		shiftAWorkTime := state.canAddWorkTime(ua, -timeWorthA+timeWorthB)
		shiftBWorkTime := state.canAddWorkTime(ub, -timeWorthB+timeWorthA)

		if (!shiftAWorkTime || !shiftBWorkTime) && rand.Float64() < 0.75 {
			continue
		}

		break
	}

	newState := state.clone()

	newState.staffAssignments[ua][sha] = shiftB
	newState.plannedWorkTimes[ua] -= (time.Duration(state.schedule[shiftA.Date][shiftA.ShiftID].MinutesWorth) * time.Minute)
	newState.plannedWorkTimes[ua] += (time.Duration(state.schedule[shiftB.Date][shiftB.ShiftID].MinutesWorth) * time.Minute)

	newState.staffAssignments[ub][shb] = shiftA
	newState.plannedWorkTimes[ub] -= (time.Duration(state.schedule[shiftB.Date][shiftB.ShiftID].MinutesWorth) * time.Minute)
	newState.plannedWorkTimes[ub] += (time.Duration(state.schedule[shiftA.Date][shiftA.ShiftID].MinutesWorth) * time.Minute)

	log.Printf("swapping shifts between %s (%s) and %s (%s)", ua, newState.diffWorkTime(ua), ub, newState.diffWorkTime(ub))

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

func (state *GeneratorState) canAddWorkTime(staff string, worktime time.Duration) bool {
	diff := (state.plannedWorkTimes[staff] - state.expectedWorkTimes[staff] + worktime)
	return diff <= 0
}

func (state *GeneratorState) diffWorkTime(staff string) string {
	return (state.plannedWorkTimes[staff] - state.expectedWorkTimes[staff]).String()
}

func (state *GeneratorState) transferShift() *GeneratorState {
	var (
		sourceStaff      string
		destinationStaff string
		lenSource        int
		sh               int
		timeWorth        time.Duration
	)
	for {
		// get some random users
		var source, destination int
		source, destination = rand.Intn(len(state.Users)), rand.Intn(len(state.Users))

		if source == destination {
			continue
		}

		sourceStaff, destinationStaff = state.Users[source].Name, state.Users[destination].Name

		if len(state.staffAssignments[destinationStaff]) > len(state.staffAssignments[sourceStaff]) {
			sourceStaff, destinationStaff = destinationStaff, sourceStaff
		}

		lenSource = len(state.staffAssignments[sourceStaff])
		sh = rand.Intn(lenSource)

		// make sure that destinationStaff is actually eligible for the shift we want
		// to transfer from sourceStaff
		shiftSource := state.staffAssignments[sourceStaff][sh]
		shift := state.schedule[shiftSource.Date][shiftSource.ShiftID]
		if !stringInSlice(destinationStaff, shift.EligibleStaff) {
			continue
		}

		timeWorth = time.Duration(shift.MinutesWorth) * time.Minute
		if !state.canAddWorkTime(destinationStaff, timeWorth) && rand.Float64() < 0.75 {
			continue
		}

		break
	}

	newState := state.clone()
	newState.staffAssignments[destinationStaff] = append(newState.staffAssignments[destinationStaff], newState.staffAssignments[sourceStaff][sh])
	newState.staffAssignments[sourceStaff][sh] = newState.staffAssignments[sourceStaff][lenSource-1]
	newState.staffAssignments[sourceStaff] = newState.staffAssignments[sourceStaff][:lenSource-1]
	newState.plannedWorkTimes[sourceStaff] -= timeWorth
	newState.plannedWorkTimes[destinationStaff] += timeWorth

	log.Printf("transfering shift from %s (%s) to %s (%s", sourceStaff, newState.diffWorkTime(sourceStaff), destinationStaff, newState.diffWorkTime(destinationStaff))

	return newState
}

func (state *TabuState) Neighbor() hego.TabuState {
	if rand.Float64() < 0.75 {
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
		RosterMeta: structs.RosterMeta{
			Month: state.month,
			Year:  state.year,
		},
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
