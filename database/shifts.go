package database

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DatabaseImpl) SaveWorkShift(ctx context.Context, workShift *structs.WorkShift) error {
	if workShift.ID.IsZero() {
		workShift.ID = primitive.NewObjectID()

		db.logger.Info("Inserting new working shift", "name", workShift.Name, "from", workShift.From, "duration", time.Duration(workShift.Duration).String(), "required-staff-count", workShift.RequiredStaffCount)

		res, err := db.shifts.InsertOne(ctx, workShift)
		if err != nil {
			return fmt.Errorf("failed to insert: %w", err)
		}

		workShift.ID = res.InsertedID.(primitive.ObjectID)

		return nil
	}

	db.logger.Info("Replacing working shift", "id", workShift.ID.Hex(), "name", workShift.Name, "from", workShift.From, "duration", time.Duration(workShift.Duration).String(), "required-staff-count", workShift.RequiredStaffCount)
	res, err := db.shifts.ReplaceOne(ctx, bson.M{"_id": workShift.ID}, workShift)
	if err != nil {
		return fmt.Errorf("failed to replace document with id %s: %w", workShift.ID, err)
	}

	if res.ModifiedCount != 1 {
		return fmt.Errorf("failed to replace document with id %s: %w", workShift.ID, mongo.ErrNoDocuments)
	}

	return nil
}

func (db *DatabaseImpl) GetShiftsForDay(ctx context.Context, weekDay time.Weekday, isHoliday bool) ([]structs.WorkShift, error) {
	filter := bson.M{
		"days":      weekDay,
		"onHoliday": isHoliday,
		"$or": bson.A{
			bson.M{"deleted": false},
			bson.M{"deleted": bson.M{
				"$exists": false,
			}},
		},
	}

	shifts, err := db.shifts.Find(ctx, filter, options.Find().SetSort(bson.M{
		"order": 1,
	}))

	if err != nil {
		return nil, err
	}

	var workShifts []structs.WorkShift
	if err := shifts.All(ctx, &workShifts); err != nil {
		return nil, err
	}

	return workShifts, nil
}

func (db *DatabaseImpl) DeleteWorkShift(ctx context.Context, id string) error {
	objId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.shifts.UpdateOne(ctx, bson.M{"_id": objId}, bson.M{
		"$set": bson.M{
			"deleted": true,
		},
	})
	if err != nil {
		return err
	}

	if res.ModifiedCount != 1 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (db *DatabaseImpl) ListWorkShifts(ctx context.Context) ([]structs.WorkShift, error) {
	res, err := db.shifts.Find(ctx, bson.D{}, options.Find().SetSort(bson.M{
		"order": 1,
	}))
	if err != nil {
		return nil, err
	}

	var result []structs.WorkShift

	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}
