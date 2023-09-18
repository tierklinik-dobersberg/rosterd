package structs

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/cis/pkg/daytime"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/durationpb"
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
		Deleted            bool               `bson:"deleted"`
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

func (shift WorkShift) ToProto() *rosterv1.WorkShift {
	protoShift := &rosterv1.WorkShift{
		Id:                 shift.ID.Hex(),
		From:               shift.From.ToProto(),
		Duration:           durationpb.New(time.Duration(shift.Duration)),
		Name:               shift.Name,
		DisplayName:        shift.ShortName,
		OnHoliday:          shift.OnHoliday,
		EligibleRoleIds:    shift.EligibleRoles,
		RequiredStaffCount: int64(shift.RequiredStaffCount),
		Color:              shift.Color,
		Description:        shift.Description,
		Order:              int64(shift.Order),
		Tags:               shift.Tags,
	}

	protoShift.Days = make([]int32, len(shift.Days))
	for idx, day := range shift.Days {
		protoShift.Days[idx] = int32(day)
	}

	if shift.MinutesWorth != nil {
		protoShift.TimeWorth = durationpb.New((time.Duration(*shift.MinutesWorth) * time.Minute))
	}

	return protoShift
}

func (dt Daytime) ToProto() *rosterv1.Daytime {
	duration := float64(time.Duration(dt))

	hours := math.Floor(duration / float64(time.Hour))
	minutes := math.Floor(float64((duration - (hours * float64(time.Hour))) / float64(time.Minute)))

	return &rosterv1.Daytime{
		Hour:   int64(hours),
		Minute: int64(minutes),
	}
}

func (dt *Daytime) FromProto(protoDayTime *rosterv1.Daytime) {
	hour := protoDayTime.Hour
	minute := protoDayTime.Minute

	duration := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute

	*dt = Daytime(duration)
}
