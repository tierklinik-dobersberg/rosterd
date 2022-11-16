package structs

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	RosterShift struct {
		Staff              []string           `json:"staff" bson:"staff"`
		ShiftID            primitive.ObjectID `json:"shiftID" bson:"shiftID"`
		IsHoliday          bool               `json:"isHoliday" bson:"isHoliday"`
		IsWeekend          bool               `json:"isWeekend" bson:"isWeekend"`
		From               time.Time          `json:"from" bson:"from"`
		To                 time.Time          `json:"to" bson:"to"`
		MinutesWorth       float64            `json:"minutesWorth" bson:"minutesWorth"`
		RequiredStaffCount int                `json:"requiredStaffCount" bson:"requiredStaffCount"`

		Definition WorkShift `json:"definition" bson:"-"`
	}

	RosterShiftWithStaffList struct {
		RosterShift   `json:",inline"`
		EligibleStaff []string                         `json:"eligibleStaff"`
		Violations    map[string][]ConstraintViolation `json:"constraintViolations"`
	}

	Roster struct {
		ID         primitive.ObjectID `json:"id" bson:"_id"`
		Month      time.Month         `json:"month" bson:"month"`
		Year       int                `json:"year" bson:"year"`
		Shifts     []RosterShift      `json:"shifts" bson:"shifts"`
		Approved   *bool              `json:"approved" bson:"approved"`
		ApprovedAt *time.Time         `json:"approvedAt" bson:"approvedAt"`
	}

	Diagnostic struct {
		Type        string `json:"type,omitempty"`
		Date        string `json:"date,omitempty"`
		Description string `json:"description,omitempty"`
		Details     any    `json:"details,omitempty"`
		Penalty     int    `json:"penalty,omitempty"`
	}

	RosterAnalysis struct {
		Diagnostics []Diagnostic               `json:"diagnostics"`
		WorkTime    map[string]*WorkTimeStatus `json:"workTime"`
		Penalty     int                        `json:"penalty"`
	}
)
