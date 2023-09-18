package database

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (db *DatabaseImpl) GetOffTimeRequest(ctx context.Context, ids ...string) ([]structs.OffTimeEntry, error) {
	var objids = make([]primitive.ObjectID, len(ids))

	for idx, id := range ids {
		obid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}

		objids[idx] = obid
	}

	res, err := db.offTime.Find(ctx, bson.M{"_id": bson.M{"$in": objids}})
	if err != nil {
		return nil, err
	}

	if res.Err() != nil {
		return nil, err
	}

	var result []structs.OffTimeEntry
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeEntry) error {
	res, err := db.offTime.InsertOne(ctx, req)
	if err != nil {
		return err
	}

	req.ID = res.InsertedID.(primitive.ObjectID)

	return nil
}

func (db *DatabaseImpl) DeleteOffTimeRequest(ctx context.Context, ids ...string) error {
	objids := make([]primitive.ObjectID, len(ids))

	for idx, id := range ids {
		obid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objids[idx] = obid
	}

	res, err := db.offTime.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": objids}})
	if err != nil {
		return err
	}

	if res.DeletedCount != int64(len(objids)) {
		return fmt.Errorf("failed to delete one or more offtime request")
	}

	return nil
}

func (db *DatabaseImpl) FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string) ([]structs.OffTimeEntry, error) {
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
		filter["$or"] = bson.A{
			fromFilter,
			toFilter,
			bson.M{
				"from": bson.M{
					"$gte": from,
				},
				"to": bson.M{
					"$lte": to,
				},
			},
		}

	case fromFilter != nil:
		filter = fromFilter
	case toFilter != nil:
		filter = toFilter
	}

	if approved != nil {
		filter["approval"] = bson.M{
			"$exists": true,
		}
		filter["approval.approved"] = *approved
	}

	if len(staff) > 0 {
		filter["requestorId"] = bson.M{
			"$in": staff,
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

func (db *DatabaseImpl) AddOffTimeCost(ctx context.Context, costs *structs.OffTimeCosts) error {
	costs.ID = primitive.NewObjectID()

	_, err := db.offTimeCosts.InsertOne(ctx, costs)
	if err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) DeleteOffTimeCostsByRoster(ctx context.Context, rosterID string) error {
	objID, err := primitive.ObjectIDFromHex(rosterID)
	if err != nil {
		return err
	}

	_, err = db.offTimeCosts.DeleteMany(ctx, bson.M{
		"rosterId": objID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) DeleteOffTimeCosts(ctx context.Context, ids ...string) error {
	objids := make([]primitive.ObjectID, len(ids))
	for idx, id := range ids {
		o, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objids[idx] = o
	}

	res, err := db.offTimeCosts.DeleteMany(ctx, bson.M{
		"_id": bson.M{
			"$in": objids,
		},
	})
	if err != nil {
		return err
	}
	if res.DeletedCount != int64(len(ids)) {
		return fmt.Errorf("failed to delete some off-time-cost entries")
	}

	return nil
}

func (db *DatabaseImpl) DeleteWorkTime(ctx context.Context, ids ...string) error {
	objids := make([]primitive.ObjectID, len(ids))
	for idx, id := range ids {
		o, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objids[idx] = o
	}

	res, err := db.worktime.DeleteMany(ctx, bson.M{
		"_id": bson.M{
			"$in": objids,
		},
	})
	if err != nil {
		return err
	}
	if res.DeletedCount != int64(len(ids)) {
		return fmt.Errorf("failed to delete some work-time entries")
	}

	return nil
}

func (db *DatabaseImpl) GetOffTimeCosts(ctx context.Context, user_ids ...string) ([]structs.OffTimeCosts, error) {
	var filter bson.M

	if len(user_ids) > 0 {
		filter = bson.M{
			"userId": bson.M{
				"$in": user_ids,
			},
		}
	}

	res, err := db.offTimeCosts.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	if res.Err() != nil {
		return nil, res.Err()
	}

	var results []structs.OffTimeCosts
	if err := res.All(ctx, &results); err != nil {
		return results, err
	}

	return results, nil
}
