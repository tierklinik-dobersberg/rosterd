package structs

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	JSDuration time.Duration

	// OffTimeCosts describes the vacation costs of an off-time
	// request.
	OffTimeCosts struct {
		// VacationDays is the number of vacation days that are required
		// to fullfil the off-time request. Note that the actual number
		// time-costs of a off-time requests depends on the number of
		// VacationDays and the current regular working time (time/week).
		//
		// The Duration field holds exactly that costs but needs to be
		// recalculated whenever the regular working hours change. See
		// Duration for more information.
		VacationDays float64 `json:"vacationDays" bson:"vacationDays"`

		// Duration is the duration of vacation time that is required
		// for the off-time request.
		// This is calculated by the number of required VacationDays
		// multiplied with the number of regular working hours per work-day.
		//
		// 		(VacationDays * WorkTime.TimePerWeek / 5)
		//
		// If the regular working hours (per week) of a employee
		// is changed Duration must be re-calculated for all OffTimeEntries
		// of type "vacation", "time-off" or "auto" that are still in the
		// future and have not actually been consumed by the employee.
		Duration JSDuration `json:"duration" bson:"duration"`
	}

	// Approval holds approval information for an off-time request.
	Approval struct {
		// Approved is set to true if the off-time request has been approved.
		// Requests that are not approved are ignored when calculating an the
		// current vacation credit of an employee.
		Approved bool `json:"approved" bson:"approved"`

		// ApprovedAt is set to the time the off-time request has been approved
		// my management.
		ApprovedAt time.Time `json:"approvedAt" bson:"approvedAt"`

		// Comment is an optional comment that may be set by management to
		// justify approval or rejection of an off-time request.
		Comment string `json:"comment" bson:"comment"`

		// ActualCosts is set to the actual costs of the off-time requests.
		// While this is often equal to the Costs field of an OffTimeEntry
		// it allows management to approve an off-time request but split the
		// required costs between vacation and time-off.
		ActualCosts OffTimeCosts `json:"actualCosts" bson:"actionCosts"`
	}

	// RequestType describes the type of an off-time request.
	//
	// It may be one of the following:
	//
	// - vacation: A request for vacation. That means that the employee is expecting
	//             to pay for the off-time request with the full amount of vacation days
	//             required for it.
	//
	// - time-off: A request for compensatory time-off. That means that the employee either
	//             must have enough overtime or that he/she is willing to work more time during
	//             the rest of the month.
	//
	// - auto: The employee doesn't really care if it's vacation or compensatory time-off and it's
	//         up to the management to decide.
	//
	// - credits: This is only every set by the auto-grant feature or by management and is used to
	//            grant additional vacation days to an employee.
	RequestType string

	OffTimeEntry struct {
		// ID is the ID of the OffTimeEntry in the MongoDB database collection.
		ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

		// From holds the time at which the employee requests off-time or after which
		// credits given by management apply (only if RequestType == "credits")
		From time.Time `json:"from" bson:"from"`

		// To holds the time until which the employee requests off-time. This field
		// is left empty (zero-time) for RequestType == "credits"
		To time.Time `json:"to" bson:"to"`

		// Description may holds an optional description about the off-time request.
		Description string `json:"description" bson:"description"`

		// StaffID is the identifier of the employee for which this off-time request
		// is. Note that employees can only ever request off-time for themselves but
		// management may create off-time requests on behave of an employee for auditing
		// and history-keeping purposes.
		StaffID string `json:"staffID" bson:"staffID"`

		// RequestType is the request type of this off-time request. See RequestType for
		// available types and their meaning.
		RequestType RequestType `json:"requestType" bson:"requestType"`

		// CreatedAt holds the time at which the request has been created.
		CreatedAt time.Time `json:"createdAt" bson:"createdAt"`

		// CreatedBy holds the name of the user that created this request.
		// Normally this is the same as staffID but may be set to the name of
		// a management user if the off-time request has been created on behave
		// of another user.
		CreatedBy string `json:"createdBy" bson:"createdBy"`

		// Cost are the time and vacation costs for this request.
		Costs OffTimeCosts `json:"costs" bson:"costs"`

		// Approval holds information about the approval of this request.
		// If Approval is still nil this request has neither been approved
		// nor rejected by management and must not be considered for overtime and
		// vacation credit calculations.
		Approval *Approval `json:"approval" bson:"approval"`
	}

	// CreateOffTimeRequest describes the JSON payload sent when creating a new
	// off-time request. Fields have the exact same meaning as their OffTimeEntry
	// counterparts.
	CreateOffTimeRequest struct {
		From        time.Time   `json:"from"`
		To          time.Time   `json:"to"`
		StaffID     string      `json:"staff"`
		Description string      `json:"description"`
		RequestType RequestType `json:"requestType"`
	}

	CreateOffTimeCreditsRequest struct {
		StaffID     string    `json:"staff"`
		From        time.Time `json:"from"`
		Description string    `json:"description"`
		Days        float64   `json:"days"`
	}
)

const (
	RequestTypeAuto     = RequestType("auto")
	RequestTypeVacation = RequestType("vacation")
	RequestTypeTimeOff  = RequestType("time-off")
	RequestTypeCredits  = RequestType("credits")
)

func (d JSDuration) MarshalJSON() ([]byte, error) {
	f := time.Duration(d) / time.Millisecond
	return json.Marshal(f)
}

func (d *JSDuration) UnmarshalJSON(blob []byte) error {
	var f time.Duration
	if err := json.Unmarshal(blob, &f); err != nil {
		return err
	}

	*d = JSDuration(f * time.Millisecond)

	return nil
}
