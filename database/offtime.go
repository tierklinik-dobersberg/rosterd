package database

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (db *DatabaseImpl) GetOffTimeRequest(ctx context.Context, id string) (*structs.OffTimeRequest, error) {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := db.offTime.FindOne(ctx, bson.M{"_id": obid})
	if res.Err() != nil {
		return nil, err
	}

	var result structs.OffTimeRequest
	if err := res.Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (db *DatabaseImpl) CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeRequest) error {
	req.ID = primitive.NewObjectID()
	req.CreatedAt = time.Now()

	res, err := db.offTime.InsertOne(ctx, req)
	if err != nil {
		return err
	}

	req.ID = res.InsertedID.(primitive.ObjectID)

	return nil
}

func (db *DatabaseImpl) DeleteOffTimeRequest(ctx context.Context, id string) error {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.offTime.DeleteOne(ctx, bson.M{"_id": obid})
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return fmt.Errorf("failed to delete offtime request")
	}

	return nil
}

func (db *DatabaseImpl) FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string) ([]structs.OffTimeRequest, error) {
	filter := bson.M{}

	if !from.IsZero() {
		filter["from"] = bson.M{
			"$ge": from,
		}
	}

	if !to.IsZero() {
		filter["to"] = bson.M{
			"$le": to,
		}
	}

	if approved != nil {
		filter["approved"] = *approved
	}

	if len(staff) > 0 {
		filter["staffID"] = bson.M{
			"$in": staff,
		}
	}

	res, err := db.offTime.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []structs.OffTimeRequest
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) UpdateOffTimeRequest(ctx context.Context, upd structs.OffTimeRequestUpdate) error {
	res, err := db.offTime.UpdateOne(ctx, bson.M{"_id": upd.ID}, bson.M{
		"$set": upd,
	})

	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("failed to update request: not found")
	}

	if res.ModifiedCount != 1 {
		return fmt.Errorf("failed to update request: already approved")
	}

	return nil
}

func (db *DatabaseImpl) ApproveOffTimeRequest(ctx context.Context, id string, approved bool) error {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.offTime.UpdateOne(ctx, bson.M{"_id": obid}, bson.M{
		"$set": bson.M{
			"approved":   approved,
			"approvedAt": time.Now(),
		},
	})

	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("failed to approved request: not found")
	}

	if res.ModifiedCount != 1 {
		return fmt.Errorf("failed to approved request: already approved")
	}

	return nil
}
