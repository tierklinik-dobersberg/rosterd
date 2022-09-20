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

	var appliesTo []string
	for _, s := range staffs {
		appliesTo = append(appliesTo, fmt.Sprintf("staff:%s", s))
	}
	for _, r := range roles {
		appliesTo = append(appliesTo, fmt.Sprintf("role:%s", r))
	}

	if len(appliesTo) > 0 {
		filter["$or"] = bson.A{
			bson.M{"appliesTo": bson.M{"$in": appliesTo}},
			bson.M{"appliesTo": bson.M{"$eq": nil}},
		}
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
