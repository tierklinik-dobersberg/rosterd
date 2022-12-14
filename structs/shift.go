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
		From               Daytime            `json:"from" bson:"from" hcl:"from"`
		Duration           JSDuration         `json:"duration" bson:"duration" hcl:"to"`
		ID                 primitive.ObjectID `json:"id" bson:"_id"`
		Days               []time.Weekday     `json:"days" bson:"days" hcl:"days"`
		ShortName          string             `json:"shortName" bson:"shortName"`
		Name               string             `json:"name" bson:"name" hcl:",label"`
		OnHoliday          bool               `json:"onHoliday" bson:"onHoliday" hcl:"onHoliday"`
		EligibleRoles      []string           `json:"eligibleRoles" bson:"eligibleRoles,omitempty" hcl:"eligibleRoles"`
		MinutesWorth       *int               `json:"minutesWorth,omitempty" bson:"minutesWorth,omitempty" hcl:"minutesWorth"`
		RequiredStaffCount int                `json:"requiredStaffCount" bson:"requiredStaffCount" hcl:"requiredStaffCount"`
		Color              string             `json:"color" bson:"color" hcl:"color"`
		Description        string             `json:"description" bson:"description"`
		Order              int                `json:"order" bson:"order"`
		Tags               []string           `json:"tags" bson:"tags"`
	}
)

func (ws WorkShift) AtDay(t time.Time) (time.Time, time.Time) {
	fromDt := time.Duration(ws.From)

	from := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Add(fromDt)

	to := from.Add(time.Duration(ws.Duration))

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
