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

func (db *DatabaseImpl) SaveRosterType(ctx context.Context, model structs.RosterType) error {
	_, err := db.dutyRosterTypes.ReplaceOne(ctx, bson.M{"unique_name": model.UniqueName}, model, options.Replace().SetUpsert(true))
	if err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) DeleteRosterType(ctx context.Context, rosterTypeName string) error {
	res, err := db.dutyRosterTypes.DeleteOne(ctx, bson.M{
		"unique_name": rosterTypeName,
	})

	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (db *DatabaseImpl) GetRosterType(ctx context.Context, name string) (structs.RosterType, error) {
	res := db.dutyRosterTypes.FindOne(ctx, bson.M{"unique_name": name})

	if res.Err() != nil {
		return structs.RosterType{}, res.Err()
	}

	var result structs.RosterType
	if err := res.Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}
func (db *DatabaseImpl) GetRosterTypes(ctx context.Context) ([]structs.RosterType, error) {
	res, err := db.dutyRosterTypes.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	var result []structs.RosterType
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) SaveDutyRoster(ctx context.Context, roster *structs.DutyRoster) (bool, error) {
	if roster.ID.IsZero() {
		roster.ID = primitive.NewObjectID()
	}

	filter := bson.M{
		"_id": roster.ID,
	}

	res, err := db.dutyRosters.ReplaceOne(ctx, filter, roster, options.Replace().SetUpsert(true))
	if err != nil {
		return false, err
	}

	if res.ModifiedCount == 0 && res.UpsertedCount == 0 {
		return false, nil
	}

	return true, nil
}

func (db *DatabaseImpl) DeleteDutyRoster(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.dutyRosters.DeleteOne(ctx, bson.M{
		"_id": oid,
	})
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (db *DatabaseImpl) ApproveDutyRoster(ctx context.Context, rosterID string, approverID string) error {
	oid, err := primitive.ObjectIDFromHex(rosterID)
	if err != nil {
		return err
	}

	res, err := db.dutyRosters.UpdateOne(
		ctx,
		bson.M{"_id": oid}, // filter
		bson.M{"$set": bson.M{
			"approved":         true,
			"approved_at":      time.Now(),
			"approver_user_id": approverID,
		}},
	)
	if err != nil {
		return err
	}

	if res.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (db *DatabaseImpl) DutyRosterByID(ctx context.Context, id string) (structs.DutyRoster, error) {
	var result structs.DutyRoster

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return result, err
	}

	res := db.dutyRosters.FindOne(ctx, bson.M{"_id": oid})
	if res.Err() != nil {
		return result, res.Err()
	}

	if err := res.Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}

func (db *DatabaseImpl) LoadDutyRosters(ctx context.Context) ([]structs.DutyRoster, error) {
	res, err := db.dutyRosters.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	var result []structs.DutyRoster
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) DutyRostersByTime(ctx context.Context, t time.Time) ([]structs.DutyRoster, error) {
	res, err := db.dutyRosters.Aggregate(ctx, mongo.Pipeline{
		{
			{
				Key: "$match",
				Value: bson.M{
					"$and": bson.A{
						bson.M{
							"$expr": bson.M{
								"$lte": bson.A{
									bson.M{
										"$dateFromString": bson.M{
											"dateString": "$from",
											"format":     "%Y-%m-%d",
										},
									},
									t,
								},
							},
						},
						bson.M{
							"$expr": bson.M{
								"$gte": bson.A{
									bson.M{
										"$dateFromString": bson.M{
											"dateString": "$to",
											"format":     "%Y-%m-%d",
										},
									},
									t,
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if res.Err() != nil {
		return nil, res.Err()
	}

	var results []structs.DutyRoster
	if err := res.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// DEPRECATED
//

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
