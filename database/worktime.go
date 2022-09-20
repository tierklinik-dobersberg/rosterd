package database

import (
	"context"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DatabaseImpl) SaveWorkTimePerWeek(ctx context.Context, wt *structs.WorkTime) error {
	wt.ID = primitive.NewObjectID()

	_, err := db.worktime.InsertOne(ctx, wt)

	return err
}

func (db *DatabaseImpl) WorkTimeHistoryForStaff(ctx context.Context, staff string) ([]structs.WorkTime, error) {
	filter := bson.M{
		"staff": staff,
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

func (db *DatabaseImpl) GetCurrentWorkTimes(ctx context.Context, until time.Time) (map[string]structs.WorkTime, error) {
	if until.IsZero() {
		until = time.Now()
	}

	res, err := db.worktime.Aggregate(ctx, bson.A{
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
				"_id": "$staff",
				"timePerWeek": bson.M{
					"$last": "$timePerWeek",
				},
				"applicableFrom": bson.M{
					"$last": "$applicableFrom",
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	var result []struct {
		Staff          string        `bson:"_id"`
		TimePerWeek    time.Duration `bson:"timePerWeek"`
		ApplicableFrom time.Time     `bson:"applicableFrom"`
	}

	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	var m = make(map[string]structs.WorkTime)
	for _, r := range result {
		m[r.Staff] = structs.WorkTime{
			Staff:          r.Staff,
			TimePerWeek:    r.TimePerWeek,
			ApplicableFrom: r.ApplicableFrom,
		}
	}

	return m, nil
}
