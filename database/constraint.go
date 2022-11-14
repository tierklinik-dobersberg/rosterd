package database

import (
	"context"
	"fmt"

	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (db *DatabaseImpl) CreateConstraint(ctx context.Context, constraint *structs.Constraint) error {
	constraint.ID = primitive.NewObjectID()
	if _, err := db.constraints.InsertOne(ctx, constraint); err != nil {
		return err
	}

	return nil
}

func (db *DatabaseImpl) UpdateConstraint(ctx context.Context, constraint *structs.Constraint) error {
	res, err := db.constraints.ReplaceOne(ctx, bson.M{"_id": constraint.ID}, constraint)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

func (db *DatabaseImpl) DeleteConstraint(ctx context.Context, id string) error {
	obid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	res, err := db.constraints.DeleteOne(ctx, bson.M{"_id": obid})
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

func (db *DatabaseImpl) FindConstraints(ctx context.Context, staffs []string, roles []string) ([]structs.Constraint, error) {
	filter := bson.M{}

	appliesToRole := bson.M{
		"appliesToRole": bson.M{"$in": roles},
	}

	appliesToUser := bson.M{
		"appliesToUser": bson.M{"$in": staffs},
	}

	switch {
	case len(staffs) > 0 && len(roles) > 0:
		filter = bson.M{
			"$or": bson.A{
				appliesToRole,
				appliesToUser,
			},
		}
	case len(staffs) > 0:
		filter = appliesToUser
	case len(roles) > 0:
		filter = appliesToRole
	}

	res, err := db.constraints.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var results []structs.Constraint
	if err := res.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
