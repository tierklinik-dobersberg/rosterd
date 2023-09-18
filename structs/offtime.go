package structs

import (
	"encoding/json"
	"time"

	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	JSDuration time.Duration

	// OffTimeCosts describes the vacation costs of an off-time
	// request.
	OffTimeCosts struct {
		ID         primitive.ObjectID `bson:"_id"`
		UserID     string             `bson:"userId"`
		OfftimeID  primitive.ObjectID `bson:"offtimeId"`
		RosterID   primitive.ObjectID `bson:"rosterId"`
		CreatedAt  time.Time          `bson:"createdAt"`
		CreatorId  string             `bson:"creatorId"`
		Costs      time.Duration      `bson:"costs"`
		IsVacation bool               `bson:"isVacation"`
		Date       time.Time          `bson:"date"`
		Comment    string             `bson:"comment"`
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

		// ApproverID holds the ID of the user that approved the request.
		ApproverID string `json:"approverId" bson:"approverId"`

		// Comment is an optional comment that may be set by management to
		// justify approval or rejection of an off-time request.
		Comment string `json:"comment" bson:"comment"`
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

		// RequestorId is the identifier of the employee for which this off-time request
		// is. Note that employees can only ever request off-time for themselves but
		// management may create off-time requests on behave of an employee for auditing
		// and history-keeping purposes.
		RequestorId string `json:"requestorId" bson:"requestorId"`

		// RequestType is the request type of this off-time request. See RequestType for
		// available types and their meaning.
		RequestType RequestType `json:"requestType" bson:"requestType"`

		// CreatedAt holds the time at which the request has been created.
		CreatedAt time.Time `json:"createdAt" bson:"createdAt"`

		// CreatorId holds the name of the user that created this request.
		// Normally this is the same as staffID but may be set to the name of
		// a management user if the off-time request has been created on behave
		// of another user.
		CreatorId string `json:"creatorId" bson:"creatorId"`

		// Approval holds information about the approval of this request.
		// If Approval is still nil this request has neither been approved
		// nor rejected by management and must not be considered for overtime and
		// vacation credit calculations.
		Approval *Approval `json:"approval" bson:"approval"`
	}
)

const (
	RequestTypeAuto     = RequestType("auto")
	RequestTypeVacation = RequestType("vacation")
	RequestTypeTimeOff  = RequestType("time-off")
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

func (requestType RequestType) ToProto() rosterv1.OffTimeType {
	switch requestType {
	case RequestTypeTimeOff:
		return rosterv1.OffTimeType_OFF_TIME_TYPE_TIME_OFF
	case RequestTypeVacation:
		return rosterv1.OffTimeType_OFF_TIME_TYPE_VACATION
	case RequestTypeAuto:
		fallthrough
	default:
		return rosterv1.OffTimeType_OFF_TIME_TYPE_UNSPECIFIED
	}
}

func (approval *Approval) ToProto() *rosterv1.OffTimeApproval {
	if approval == nil {
		return nil
	}

	return &rosterv1.OffTimeApproval{
		Approved:   approval.Approved,
		ApprovedAt: timestamppb.New(approval.ApprovedAt),
		ApproverId: approval.ApproverID,
		Comment:    approval.Comment,
	}
}

func (entry OffTimeEntry) ToProto() *rosterv1.OffTimeEntry {
	protoEntry := &rosterv1.OffTimeEntry{
		Id:          entry.ID.Hex(),
		From:        timestamppb.New(entry.From),
		Description: entry.Description,
		Type:        entry.RequestType.ToProto(),
		CreatedAt:   timestamppb.New(entry.CreatedAt),
		Approval:    entry.Approval.ToProto(),
		RequestorId: entry.RequestorId,
		To:          timestamppb.New(entry.To),
		CreatorId:   entry.CreatorId,
	}

	return protoEntry
}
