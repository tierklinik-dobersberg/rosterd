package structs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/cis/pkg/daytime"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	// Daytime represents a time during any day. It is encoded as the duration
	// from midnight. Note that this makes Daytime timezone specific so it is
	// important to always use the same timezone when calculating and actual
	// time.Time.
	//
	// An example would be:
	//
	//	var dt = Daytime(8 * time.Hour + 30 * time.Minute) // 08:30
	//	daytime.Midnight(time.Now()).Add()
	//
	Daytime time.Duration

	WorkShift struct {
		From               Daytime            `json:"from" bson:"from"`
		Duration           time.Duration      `json:"duration" bson:"duration"`
		ID                 primitive.ObjectID `json:"id" bson:"_id"`
		Days               []time.Weekday     `json:"days" bson:"days"`
		Name               string             `json:"name" bson:"name"`
		OnHoliday          bool               `json:"onHoliday" bson:"onHoliday"`
		EligibleRoles      []string           `json:"eligibleRoles" bson:"eligibleRoles,omitempty"`
		MinutesWorth       *int               `json:"minutesWorth" bson:"minutesWorth,omitempty"`
		RequiredStaffCount int                `json:"requiredStaffCount" bson:"requiredStaffCount"`
	}

	RosterShift struct {
		Staff        []string           `json:"staff" bson:"staff"`
		ShiftID      primitive.ObjectID `json:"shiftID" bson:"shiftID"`
		Name         string             `json:"name" bson:"name"`
		IsHoliday    bool               `json:"isHoliday" bson:"isHoliday"`
		IsWeekend    bool               `json:"isWeekend" bson:"isWeekend"`
		From         time.Time          `json:"from" bson:"from"`
		To           time.Time          `json:"to" bson:"to"`
		MinutesWorth float64            `json:"minutesWorth" bson:"minutesWorth"`
	}

	Roster struct {
		ID         primitive.ObjectID `json:"id" bson:"_id"`
		Shifts     []RosterShift      `json:"shifts" bson:"shifts"`
		Approved   *bool              `json:"approved" bson:"approved"`
		ApprovedAt time.Time          `json:"approvedAt" bson:"approvedAt"`
	}

	OffTimeRequest struct {
		ID               primitive.ObjectID `json:"id" bson:"_id"`
		From             time.Time          `json:"from" bson:"from"`
		To               time.Time          `json:"to" bson:"to"`
		Description      string             `json:"description" bson:"description"`
		StaffID          string             `json:"staffID" bson:"staffID"`
		IsSoftConstraint bool               `json:"isSoftConstraint" bson:"isSoftConstraint"`
		Approved         *bool              `json:"approved" bson:"approved"`
		ApprovedAt       time.Time          `json:"approvedAt" bson:"approvedAt"`
		CreatedAt        time.Time          `json:"createdAt" bson:"createdAt"`
	}

	OffTimeRequestUpdate struct {
		ID               primitive.ObjectID `json:"id" bson:"_id"`
		From             *time.Time         `json:"from" bson:"from"`
		To               *time.Time         `json:"to" bson:"to"`
		StaffID          *string            `json:"staffID" bson:"staffID"`
		IsSoftConstraint *bool              `json:"isSoftConstraint" bson:"isSoftConstraint"`
		Approved         *bool              `json:"approved" bson:"approved"`
		ApprovedAt       *time.Time         `json:"approvedAt" bson:"approvedAt"`
		CreatedAt        *time.Time         `json:"createdAt" bson:"createdAt"`
	}
)

func (ws WorkShift) AtDay(t time.Time) (time.Time, time.Time) {
	fromDt := time.Duration(ws.From)

	from := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Add(fromDt)

	to := from.Add(ws.Duration)

	return from, to
}

func (dt Daytime) String() string {
	hours := time.Duration(dt) / time.Hour
	minutes := time.Duration(dt)/time.Minute - 60*hours

	return fmt.Sprintf("%02d:%02d", int(hours), int(minutes))
}

func (dt Daytime) MarshalJSON() ([]byte, error) {
	return json.Marshal(dt.String())
}

func (dt *Daytime) UnmarshalJSON(blob []byte) error {
	var s string
	if err := json.Unmarshal(blob, &s); err != nil {
		return err
	}

	res, err := daytime.ParseDayTime(s)
	if err != nil {
		return err
	}

	*dt = Daytime(res.AsDuration())

	return nil
}
