package database

import (
	"context"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DatabaseImpl) SaveWorkTimePerWeek(ctx context.Context, wt *structs.WorkTime) error {
	wt.ID = primitive.NewObjectID()

	_, err := db.worktime.InsertOne(ctx, wt)

	return err
}

func (db *DatabaseImpl) UpdateWorkTime(ctx context.Context, wt *structs.WorkTime) error {
	res, err := db.worktime.ReplaceOne(ctx, bson.M{"_id": wt.ID}, wt)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (db *DatabaseImpl) WorkTimeHistoryForStaff(ctx context.Context, userID string) ([]structs.WorkTime, error) {
	filter := bson.M{
		"userID": userID,
	}

	res, err := db.worktime.Find(ctx, filter, options.Find().SetSort(bson.D{
		{Key: "applicableFrom", Value: 1},
	}))
	if err != nil {
		return nil, err
	}

	var result []structs.WorkTime
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) GetWorktimeByID(ctx context.Context, id string) (*structs.WorkTime, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := db.worktime.FindOne(ctx, bson.M{
		"_id": oid,
	})

	if res.Err() != nil {
		return nil, err
	}

	var result structs.WorkTime
	if err := res.Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (db *DatabaseImpl) GetCurrentWorkTimes(ctx context.Context, until time.Time) (map[string]structs.WorkTime, error) {
	if until.IsZero() {
		until = time.Now()
	}

	pipeline := bson.A{
		bson.M{
			"$match": bson.M{
				"applicableFrom": bson.M{
					"$lte": until,
				},
			},
		},
		// first we need to sort the results
		bson.M{
			"$sort": bson.M{
				"applicableFrom": 1,
			},
		},
		// Next, group them by staff id
		bson.M{
			"$group": bson.M{
				"_id": "$userID",
				"workTimeID": bson.M{
					"$last": "$_id",
				},
				"timePerWeek": bson.M{
					"$last": "$timePerWeek",
				},
				"applicableFrom": bson.M{
					"$last": "$applicableFrom",
				},
				"endsWith": bson.M{
					"$last": "$endsWith",
				},
				"excludeFromTimeTracking": bson.M{
					"$last": "$excludeFromTimeTracking",
				},
				"vacationWeeksPerYear": bson.M{
					"$last": "$vacationWeeksPerYear",
				},
			},
		},
	}

	res, err := db.worktime.Aggregate(ctx, pipeline)

	if err != nil {
		return nil, err
	}

	var result []struct {
		Staff                   string             `bson:"_id"`
		TimePerWeek             time.Duration      `bson:"timePerWeek"`
		ApplicableFrom          time.Time          `bson:"applicableFrom"`
		VacationWeeksPerYear    float32            `bson:"vacationWeeksPerYear"`
		WorkTimeID              primitive.ObjectID `bson:"workTimeID"`
		ExcludeFromTimeTracking bool               `bson:"excludeFromTimeTracking"`
		EndsWith                time.Time          `bson:"endsWith"`
	}

	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	var m = make(map[string]structs.WorkTime)
	for _, r := range result {
		m[r.Staff] = structs.WorkTime{
			ID:                      r.WorkTimeID,
			UserID:                  r.Staff,
			TimePerWeek:             r.TimePerWeek,
			ApplicableFrom:          r.ApplicableFrom,
			VacationWeeksPerYear:    r.VacationWeeksPerYear,
			EndsWith:                r.EndsWith,
			ExcludeFromTimeTracking: r.ExcludeFromTimeTracking,
		}
	}

	return m, nil
}
