package database

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (db *DatabaseImpl) GetOffTimeRequest(ctx context.Context, id string) (*structs.OffTimeEntry, error) {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := db.offTime.FindOne(ctx, bson.M{"_id": obid})
	if res.Err() != nil {
		return nil, err
	}

	var result structs.OffTimeEntry
	if err := res.Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (db *DatabaseImpl) CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeEntry) error {
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

func (db *DatabaseImpl) FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string, isCredit *bool) ([]structs.OffTimeEntry, error) {
	filter := bson.M{}

	var fromFilter bson.M
	if !from.IsZero() {
		fromFilter = bson.M{
			"from": bson.M{
				"$lte": from,
			},
			"to": bson.M{
				"$gte": from,
			},
		}
	}

	var toFilter bson.M
	if !to.IsZero() {
		toFilter = bson.M{
			"from": bson.M{
				"$lte": to,
			},
			"to": bson.M{
				"$gte": to,
			},
		}
	}

	switch {
	case fromFilter != nil && toFilter != nil:
		filter["$or"] = bson.A{fromFilter, toFilter}
	case fromFilter != nil:
		filter = fromFilter
	case toFilter != nil:
		filter = toFilter
	}

	if approved != nil {
		filter["approval"] = bson.M{
			"$exists": true,
		}
		filter["approval.approved"] = true
	}

	if len(staff) > 0 {
		filter["staffID"] = bson.M{
			"$in": staff,
		}
	}

	if isCredit != nil {
		if *isCredit {
			filter["requestType"] = structs.RequestTypeCredits
		} else {
			filter["duration"] = bson.M{
				"$ne": structs.RequestTypeCredits,
			}
		}
	}

	db.dumpFilter("FindOffTimeRequests", filter)

	res, err := db.offTime.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []structs.OffTimeEntry
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) ApproveOffTimeRequest(ctx context.Context, id string, approval *structs.Approval) error {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.offTime.UpdateOne(ctx, bson.M{"_id": obid}, bson.M{
		"$set": bson.M{
			"approval": approval,
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

func (db *DatabaseImpl) CalculateOffTimeCredits(ctx context.Context) (map[string]structs.JSDuration, error) {
	res, err := db.offTime.Aggregate(ctx, bson.A{
		bson.M{
			"$match": bson.M{
				"approval.approved": true,
			},
		},
		bson.M{
			"$group": bson.M{
				"_id": "$staffID",
				"durationCreditsLeft": bson.M{
					"$sum": "$approval.actualCosts.duration",
				},
				"dayCreditsLeft": bson.M{
					"$sum": "$approval.actualCosts.days",
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	var entries []struct {
		StaffID        string             `bson:"_id"`
		CreditsLeft    structs.JSDuration `bson:"durationCreditsLeft"`
		DayCreditsLeft float64            `bson:"dayCreditsLeft"`
	}

	if err := res.All(ctx, &entries); err != nil {
		return nil, err
	}

	result := make(map[string]structs.JSDuration)

	for _, e := range entries {
		result[e.StaffID] = e.CreditsLeft
	}

	return result, nil
}
