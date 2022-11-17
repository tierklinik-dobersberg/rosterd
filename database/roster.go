package database

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DatabaseImpl) CreateRoster(ctx context.Context, roster structs.Roster) error {
	roster.ID = primitive.NewObjectID()

	_, err := db.rosters.InsertOne(ctx, roster)
	if err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) UpdateRoster(ctx context.Context, roster structs.Roster) error {
	if roster.ID.IsZero() {
		return fmt.Errorf("missing roster id")
	}

	_, err := db.rosters.ReplaceOne(ctx, bson.M{"_id": roster.ID}, roster)
	if err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) FindRoster(ctx context.Context, month time.Month, year int) (*structs.Roster, error) {
	result := db.rosters.FindOne(ctx, bson.M{"month": month, "year": year})
	if result.Err() != nil {
		return nil, result.Err()
	}

	var roster structs.Roster
	if err := result.Decode(&roster); err != nil {
		return nil, err
	}

	return &roster, nil
}

func (db *DatabaseImpl) ListRosterMeta(ctx context.Context, approved *bool) ([]structs.RosterMeta, error) {
	filter := bson.M{}

	if approved != nil {
		filter["approved"] = approved
	}

	results, err := db.rosters.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{
			{Key: "year", Value: 1},
			{Key: "month", Value: 1},
		}),
	)
	if err != nil {
		return nil, err
	}

	var meta []structs.RosterMeta
	if err := results.All(ctx, &meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (db *DatabaseImpl) DeleteRoster(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	result, err := db.rosters.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

func (db *DatabaseImpl) LoadRoster(ctx context.Context, id string) (*structs.Roster, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := db.rosters.FindOne(ctx, bson.M{"_id": oid})
	if result.Err() != nil {
		return nil, result.Err()
	}

	var roster structs.Roster
	if err := result.Decode(&roster); err != nil {
		return nil, err
	}

	return &roster, nil
}

func (db *DatabaseImpl) ApproveRoster(ctx context.Context, approver string, month time.Month, year int) error {
	result, err := db.rosters.UpdateOne(
		ctx, bson.M{
			"year":  year,
			"month": month,
		},
		bson.M{
			"$set": bson.M{
				"approved":   true,
				"approvedAt": time.Now(),
				"approvedBy": approver,
			},
		},
	)

	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("not found")
	}

	if result.ModifiedCount == 0 {
		return fmt.Errorf("already approved")
	}

	return nil
}
