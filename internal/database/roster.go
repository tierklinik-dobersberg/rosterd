package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
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

func (db *DatabaseImpl) SaveDutyRoster(ctx context.Context, roster *structs.DutyRoster, casIndex *uint64) (bool, error) {
	if roster.ID.IsZero() {
		roster.ID = primitive.NewObjectID()
	}

	filter := bson.M{
		"_id": roster.ID,
	}

	// increase roster CAS index everytime we attempt to save it
	roster.CASIndex++

	opts := options.Replace()
	if casIndex != nil {
		filter["$or"] = bson.A{
			bson.M{"cas_index": *casIndex},
			bson.M{"cas_index": bson.M{
				"$exists": false,
			}},
		}
	} else {
		// If there's no CAS index given we enable upsert support.
		opts.SetUpsert(true)
	}

	res, err := db.dutyRosters.ReplaceOne(ctx, filter, roster, opts)
	if err != nil {
		return false, err
	}

	if casIndex != nil {
		if res.MatchedCount == 0 {
			return false, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("CAS index conflict"))
		}
	}

	if res.ModifiedCount == 0 && res.UpsertedCount == 0 {
		return false, nil
	}

	return true, nil
}

func (db *DatabaseImpl) DeleteDutyRoster(ctx context.Context, id string, supersededBy primitive.ObjectID) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	// if the roster is not superseded by a different one, we can just remove
	// it from the collection
	if supersededBy.IsZero() {
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

	// the roster has been superseded by a new version, this may happen if the
	// roster is changed although it has already been approved.
	// In this case, we just mark the roster as deleted and store the ID of the new roster
	// so we can calculate differences and send nices mail updates.
	res, err := db.dutyRosters.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"deleted":      true,
			"supersededBy": supersededBy,
		},
	})
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
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

func (db *DatabaseImpl) GetSupersededDutyRoster(ctx context.Context, rosterID primitive.ObjectID) (*structs.DutyRoster, error) {
	res := db.dutyRosters.FindOne(ctx, bson.M{"supersededBy": rosterID})
	if res.Err() != nil {
		return nil, res.Err()
	}

	var r structs.DutyRoster
	if err := res.Decode(&r); err != nil {
		return nil, err
	}

	return &r, nil
}

func (db *DatabaseImpl) LoadDutyRosters(ctx context.Context) ([]structs.DutyRoster, error) {
	res, err := db.dutyRosters.Find(ctx, bson.M{
		"deleted": bson.M{
			"$exists": false,
		},
	})
	if err != nil {
		return nil, err
	}

	var result []structs.DutyRoster
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (db *DatabaseImpl) FindRostersWithActiveShiftsInRange(ctx context.Context, from, to time.Time) ([]structs.DutyRoster, error) {
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
	res, err := db.dutyRosters.Find(ctx, bson.M{
		"shifts": bson.M{
			"$elemMatch": filter,
		},
		"deleted": bson.M{
			"$exists": false,
		},
	})

	if err != nil {
		return nil, err
	}

	var results []structs.DutyRoster
	if err := res.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *DatabaseImpl) FindRostersWithActiveShifts(ctx context.Context, t time.Time) ([]structs.DutyRoster, error) {
	res, err := db.dutyRosters.Find(ctx, bson.M{
		"shifts": bson.M{
			"$elemMatch": bson.M{
				"from": bson.M{
					"$lte": t,
				},
				"to": bson.M{
					"$gte": t,
				},
			},
		},
		"deleted": bson.M{
			"$exists": false,
		},
	})

	if err != nil {
		return nil, err
	}

	var results []structs.DutyRoster
	if err := res.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *DatabaseImpl) DutyRostersByTime(ctx context.Context, t time.Time) ([]structs.DutyRoster, error) {
	// make sure we use correct hours/minutes for the from/to query
	from := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	to := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, t.Location())

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
									to,
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
									from,
								},
							},
						},
						bson.M{
							"deleted": bson.M{
								"$exists": false,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {

		log.L(ctx).
			With("from", from).
			With("to", to).
			With("error", err).
			Error("failed to search for duty rosters by time")

		return nil, err
	}

	if res.Err() != nil {
		log.L(ctx).
			With("from", from).
			With("to", to).
			With("error", res.Err()).
			Error("failed to search for duty rosters by time")

		return nil, res.Err()
	}

	var results []structs.DutyRoster
	if err := res.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		log.L(ctx).
			With("from", from).
			With("to", to).
			With("error", err).
			Error("failed to search for duty rosters by time")
	}

	return results, nil
}
