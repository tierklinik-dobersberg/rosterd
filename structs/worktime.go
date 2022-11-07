package structs

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	WorkTime struct {
		ID             primitive.ObjectID `json:"id" bson:"_id"`
		Staff          string             `json:"staff" bson:"staff"`
		TimePerWeek    time.Duration      `json:"timePerWeek" bson:"timePerWeek"`
		ApplicableFrom time.Time          `json:"applicableFrom" bson:"applicableFrom"`

		VacationAutoGrantDays float64 `json:"vacationAutoGrantDays" bson:"vacationAutoGrantDays"`

		OvertimePenaltyRatio  float64 `json:"overtimePenaltyRation" bson:"overtimePenaltyRation"`
		UndertimePenaltyRatio float64 `json:"undertimePenaltyRation" bson:"undertimePenaltyRation"`
	}

	WorkTimeStatus struct {
		TimePerWeek           time.Duration `json:"timePerWeek"`
		ExpectedWorkTime      time.Duration `json:"expectedMonthlyHours"`
		PlannedWorkTime       time.Duration `json:"plannedMonthlyHours"`
		Penalty               int           `json:"penalty"`
		OvertimePenaltyRatio  float64       `json:"overtimePenaltyRation" bson:"overtimePenaltyRation"`
		UndertimePenaltyRatio float64       `json:"undertimePenaltyRation" bson:"undertimePenaltyRation"`
	}
)
