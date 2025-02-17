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
				rosterListBSON, err := d.Collection(RosterCollection).Find(ctx, bson.M{})
				if err != nil {
					return fmt.Errorf("failed to find rosters: %w", err)
				}

				var rosters []structs.DutyRoster
				if err := rosterListBSON.All(ctx, &rosters); err != nil {
					return fmt.Errorf("failed to decode rosters: %w", err)
				}

				// load all workshift definitions
				shiftListBSON, err := d.Collection(ShiftCollection).Find(ctx, bson.M{})
				if err != nil {
					return fmt.Errorf("failed to load workshift definitions: %w", err)
				}

				var shifts []structs.WorkShift
				if err := shiftListBSON.All(ctx, &shifts); err != nil {
					return fmt.Errorf("failed to decode workshift definitions: %w", err)
				}

				// create a lookup map for the workshifts by their ID
				shiftMap := make(map[string]structs.WorkShift, len(shifts))
				for _, s := range shifts {
					shiftMap[s.ID.Hex()] = s
				}

				// iterate over all rosters and their planned shifts and update the time-worth field
				for _, r := range rosters {
					slog.Info("migrating roster", "id", r.ID.Hex(), "from", r.FromTime().Format("2006-01-02"), "to", r.ToTime().Format("2006-01-02"))

					for idx, p := range r.Shifts {
						workShift, ok := shiftMap[p.WorkShiftID.Hex()]
						if !ok {
							return fmt.Errorf("failed to find workshift definition for id %q", p.WorkShiftID.Hex())
						}

						if workShift.MinutesWorth != nil {
							p.TimeWorth = time.Duration(*workShift.MinutesWorth) * time.Minute
						} else {
							p.TimeWorth = p.To.Sub(p.From)
						}

						slog.Info("  -> updating shift", "from", p.From.Format("15:04"), "to", p.To.Format("15:04"), "timeWorth", p.TimeWorth)

						r.Shifts[idx] = p
					}
				}

				return fmt.Errorf("just a test")
			}),
		},
	}

	migrator := mongomigrate.NewMigrator(db, "")

	migrator.Register(migrations...)

	return migrator.Run(ctx)
}
