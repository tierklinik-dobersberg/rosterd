package structs

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	Constraint struct {
		ID             primitive.ObjectID `json:"id" bson:"_id"`
		Expression     string             `json:"expression" bson:"expression"`
		AppliesToStaff []string           `json:"appliesToStaff" bson:"appliesToStaff"`
		AppliesToRole  []string           `json:"appliesToRole" bson:"appliesToRole"`
	}

	EvalContext struct {
		Staff   string
		Shift   WorkShift
		Day     time.Weekday
		Holiday bool
	}
)
