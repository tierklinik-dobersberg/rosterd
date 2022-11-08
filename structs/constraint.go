package structs

import (
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
	}

	ConstraintViolation struct {
		ID      primitive.ObjectID `json:"id"`
		Panalty int                `json:"penalty"`
		Type    string             `json:"type"` // offtime, constraint
		Name    string             `json:"name"`
		Hard    bool               `json:"hard"`
	}
)
