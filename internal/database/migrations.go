package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tierklinik-dobersberg/apis/pkg/mongomigrate"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func RunMigrations(ctx context.Context, db *mongo.Database) error {
	n := db.Name()

	var migrations = []mongomigrate.Migration{
		{
			Version:     1,
			Description: "Copy time-worth from the work-shift definition to each planned shift",
			Database:    n,
			Up: mongomigrate.MigrateFunc(func(ctx mongo.SessionContext, d *mongo.Database) error {

				// load all rosters
				rosterListBSON, err := d.Collection(DutyRosterCollection).Find(ctx, bson.M{})
				if err != nil {
					return fmt.Errorf("failed to find rosters: %w", err)
				}

				var rosters []structs.DutyRoster
				if err := rosterListBSON.All(ctx, &rosters); err != nil {
					return fmt.Errorf("failed to decode rosters: %w", err)
				}
				slog.Info("migrations: successfully loaded rosters from the database collection", "count", len(rosters))

				// load all workshift definitions
				shiftListBSON, err := d.Collection(ShiftCollection).Find(ctx, bson.M{})
				if err != nil {
					return fmt.Errorf("failed to load workshift definitions: %w", err)
				}

				var shifts []structs.WorkShift
				if err := shiftListBSON.All(ctx, &shifts); err != nil {
					return fmt.Errorf("failed to decode workshift definitions: %w", err)
				}
				slog.Info("migrations: successfully loaded workshift definitions from the database collection", "count", len(shifts))

				// create a lookup map for the workshifts by their ID
				shiftMap := make(map[string]structs.WorkShift, len(shifts))
				for _, s := range shifts {
					shiftMap[s.ID.Hex()] = s
				}

				// iterate over all rosters and their planned shifts and update the time-worth field
				for _, r := range rosters {
					slog.Info("migrating roster", "id", r.ID.Hex(), "from", r.FromTime().Local().Format("2006-01-02"), "to", r.ToTime().Local().Format("2006-01-02"))

					for idx, p := range r.Shifts {
						workShift, ok := shiftMap[p.WorkShiftID.Hex()]
						if !ok {
							return fmt.Errorf("failed to find workshift definition for id %q", p.WorkShiftID.Hex())
						}

						if workShift.MinutesWorth != nil && *workShift.MinutesWorth > 0 {
							p.TimeWorth = time.Duration(*workShift.MinutesWorth) * time.Minute
						} else {
							p.TimeWorth = p.To.Sub(p.From)
						}

						slog.Info("  -> updating shift", "from", p.From.Local().Format("15:04"), "to", p.To.Local().Format("15:04"), "timeWorth", p.TimeWorth, "name", workShift.Name)

						r.Shifts[idx] = p
					}

					updateResult, err := d.Collection(DutyRosterCollection).ReplaceOne(ctx, bson.M{"_id": r.ID}, r)
					if err != nil {
						return fmt.Errorf("failed to update duty roster %q in collection: %w", r.ID.Hex(), err)
					}

					if updateResult.ModifiedCount != 1 {
						return fmt.Errorf("unexpected modified-count for update operation: expected 1 got %d", updateResult.ModifiedCount)
					}
				}

				return nil
			}),
		},
	}

	migrator := mongomigrate.NewMigrator(db, "")

	migrator.Register(migrations...)

	return migrator.Run(ctx)
}
