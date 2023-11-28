package structs

import (
	"time"

	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	RosterType struct {
		UniqueName string   `bson:"unique_name"`
		ShiftTags  []string `bson:"shift_tags"`
		OnCallTags []string `bson:"on_call_tags"`
	}

	PlannedShift struct {
		From            time.Time          `bson:"from"`
		To              time.Time          `bson:"to"`
		AssignedUserIds []string           `bson:"assigned_user_ids"`
		WorkShiftID     primitive.ObjectID `bson:"work_shift_id"`
	}

	RequiredShift struct {
		From            time.Time
		To              time.Time
		WorkShiftID     primitive.ObjectID
		EligibleUserIds []string
		OnHoliday       bool
		OnWeekend       bool
		Violations      map[string]*rosterv1.ConstraintViolationList
	}

	DutyRoster struct {
		ID             primitive.ObjectID `bson:"_id"`
		From           string             `bson:"from"`
		To             string             `bson:"to"`
		Shifts         []PlannedShift     `bson:"shifts"`
		Approved       bool               `bson:"approved"`
		ApprovedAt     time.Time          `bson:"approved_at"`
		ApproverUserId string             `bson:"approver_user_id"`
		LastModifiedBy string             `bson:"last_modified_by"`
		CreatedAt      time.Time          `bson:"created_at"`
		UpdatedAt      time.Time          `bson:"updated_at"`
		ShiftTags      []string           `bson:"shift_tags"`
		RosterTypeName string             `bson:"roster_type_name"`

		Deleted      bool               `bson:"deleted,omitempty"`
		SupersededBy primitive.ObjectID `bson:"supersededBy,omitempty"`
	}
)

func (t RosterType) ToProto() *rosterv1.RosterType {
	return &rosterv1.RosterType{
		UniqueName: t.UniqueName,
		ShiftTags:  t.ShiftTags,
		OnCallTags: t.OnCallTags,
	}
}

func (p PlannedShift) ToProto() *rosterv1.PlannedShift {
	protoShift := &rosterv1.PlannedShift{
		From:            timestamppb.New(p.From),
		To:              timestamppb.New(p.To),
		AssignedUserIds: p.AssignedUserIds,
	}

	if !p.WorkShiftID.IsZero() {
		protoShift.WorkShiftId = p.WorkShiftID.Hex()
	}

	return protoShift
}

func (p *PlannedShift) FromProto(protoShift *rosterv1.PlannedShift) error {
	if protoShift.From.IsValid() {
		p.From = protoShift.From.AsTime()
	}

	if protoShift.To.IsValid() {
		p.To = protoShift.To.AsTime()
	}

	if protoShift.WorkShiftId != "" {
		var err error
		p.WorkShiftID, err = primitive.ObjectIDFromHex(protoShift.WorkShiftId)
		if err != nil {
			return err
		}
	}

	p.AssignedUserIds = protoShift.AssignedUserIds

	return nil
}

func (rs RequiredShift) ToProto() *rosterv1.RequiredShift {
	return &rosterv1.RequiredShift{
		From:                timestamppb.New(rs.From),
		To:                  timestamppb.New(rs.To),
		WorkShiftId:         rs.WorkShiftID.Hex(),
		EligibleUserIds:     rs.EligibleUserIds,
		OnHoliday:           rs.OnHoliday,
		OnWeekend:           rs.OnWeekend,
		ViolationsPerUserId: rs.Violations,
	}
}

func (r DutyRoster) ToProto() *rosterv1.Roster {
	protoRoster := &rosterv1.Roster{
		Id:             r.ID.Hex(),
		From:           r.From,
		To:             r.To,
		Approved:       r.Approved,
		ApproverUserId: r.ApproverUserId,
		LastModifiedBy: r.LastModifiedBy,
		CreatedAt:      timestamppb.New(r.CreatedAt),
		UpdatedAt:      timestamppb.New(r.UpdatedAt),
		RosterTypeName: r.RosterTypeName,
	}

	protoRoster.Shifts = make([]*rosterv1.PlannedShift, len(r.Shifts))
	for idx, shift := range r.Shifts {
		protoRoster.Shifts[idx] = shift.ToProto()
	}

	if r.IsApproved() {
		protoRoster.ApprovedAt = timestamppb.New(r.ApprovedAt)
	}

	return protoRoster
}

func (r DutyRoster) IsApproved() bool { return !r.ApprovedAt.IsZero() }

func (r DutyRoster) FromTime() time.Time {
	t, _ := time.ParseInLocation("2006-01-02", r.From, time.Local)

	return t
}

func (r DutyRoster) ToTime() time.Time {
	t, _ := time.ParseInLocation("2006-01-02", r.To, time.Local)
	t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	return t
}
