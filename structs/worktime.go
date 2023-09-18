package structs

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	WorkTime struct {
		ID                   primitive.ObjectID `json:"id" bson:"_id"`
		UserID               string             `json:"userID" bson:"userID"`
		TimePerWeek          time.Duration      `json:"timePerWeek" bson:"timePerWeek"`
		ApplicableFrom       time.Time          `json:"applicableFrom" bson:"applicableFrom"`
		VacationWeeksPerYear float32            `json:"vacationWeeksPerYear" bson:"vacationWeeksPerYear"`
	}

	WorkTimeStatus struct {
		TimePerWeek           JSDuration `json:"timePerWeek"`
		ExpectedWorkTime      JSDuration `json:"expectedWorkTime"`
		PlannedWorkTime       JSDuration `json:"plannedWorkTime"`
		Penalty               int        `json:"penalty"`
		OvertimePenaltyRatio  float64    `json:"overtimePenaltyRation" bson:"overtimePenaltyRation"`
		UndertimePenaltyRatio float64    `json:"undertimePenaltyRation" bson:"undertimePenaltyRation"`
	}
)
