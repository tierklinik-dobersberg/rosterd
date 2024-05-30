package structs

import (
	"time"

	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	Constraint struct {
		ID            primitive.ObjectID `json:"id" bson:"_id"`
		Description   string             `json:"description" bson:"description" hcl:",label"`
		Expression    string             `json:"expression" bson:"expression" hcl:"expr"`
		AppliesToRole []string           `json:"appliesToRole" bson:"appliesToRole" hcl:"appliesToRole"`
		AppliesToUser []string           `json:"appliesToUser" bson:"appliesToUser" hcl:"appliesToUser"`
		Hard          bool               `json:"hard" bson:"hard" hcl:"hard"`
		Penalty       int                `json:"penalty" bson:"penalty" hcl:"penalty"`
		Deny          bool               `json:"deny" bson:"deny" hcl:"deny"`
		RosterOnly    bool               `json:"rosterOnly" bson:"rosterOnly"`
		CreatedAt     time.Time          `bson:"createdAt"`
		CreatorId     string             `bson:"creatorId"`
		UpdatedAt     time.Time          `bson:"updatedAt"`
		LastUpdatedBy string             `bson:"lastUpdatedBy"`
	}

	ConstraintViolation struct {
		ID      primitive.ObjectID `json:"id"`
		Panalty int                `json:"penalty"`
		Type    string             `json:"type"` // offtime, constraint
		Name    string             `json:"name"`
		Hard    bool               `json:"hard"`
	}
)

func (c Constraint) ToProto() *rosterv1.Constraint {
	return &rosterv1.Constraint{
		Id:          c.ID.Hex(),
		Description: c.Description,
		Expression:  c.Expression,
		RoleIds:     c.AppliesToRole,
		UserIds:     c.AppliesToUser,
		Hard:        c.Hard,
		Penalty:     float32(c.Penalty),
		Deny:        c.Deny,
		RosterOnly:  c.RosterOnly,
	}
}
