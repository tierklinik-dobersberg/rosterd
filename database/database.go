package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ShiftCollection          = "rosterd-shifts"
	RosterCollection         = "rosterd-rosters"
	OffTimeRequestCollection = "rosterd-offtime"
	ConstraintCollection     = "rosterd-constraints"
	WorktimeCollection       = "rosterd-worktime"
)

type (
	WorkShiftDatabase interface {
		SaveWorkShift(context.Context, *structs.WorkShift) error
		DeleteWorkShift(context.Context, string) error
		ListWorkShifts(context.Context) ([]structs.WorkShift, error)
		GetShiftsForDay(ctx context.Context, weekDay time.Weekday, isHoliday bool) ([]structs.WorkShift, error)
	}

	OffTimeDatabase interface {
		GetOffTimeRequest(ctx context.Context, id string) (*structs.OffTimeEntry, error)
		CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeEntry) error
		DeleteOffTimeRequest(ctx context.Context, id string) error
		FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string, isCredit *bool) ([]structs.OffTimeEntry, error)
		ApproveOffTimeRequest(ctx context.Context, id string, approval *structs.Approval) error
		CalculateOffTimeCredits(ctx context.Context) (map[string]structs.JSDuration, error)
	}

	ConstraintDatabase interface {
		CreateConstraint(ctx context.Context, req *structs.Constraint) error
		UpdateConstraint(ctx context.Context, constraint *structs.Constraint) error
		DeleteConstraint(ctx context.Context, id string) error
		FindConstraints(ctx context.Context, staff []string, roles []string) ([]structs.Constraint, error)
	}

	WorkTimeDatabase interface {
		SaveWorkTimePerWeek(ctx context.Context, wt *structs.WorkTime) error
		WorkTimeHistoryForStaff(ctx context.Context, staff string) ([]structs.WorkTime, error)
		GetCurrentWorkTimes(ctx context.Context, until time.Time) (map[string]structs.WorkTime, error)
	}

	RosterDatabase interface {
		CreateRoster(ctx context.Context, roster structs.Roster) error
		UpdateRoster(ctx context.Context, roster structs.Roster) error
		FindRoster(ctx context.Context, month time.Month, year int) (*structs.Roster, error)
		DeleteRoster(ctx context.Context, id string) error
		LoadRoster(ctx context.Context, id string) (*structs.Roster, error)
		ApproveRoster(ctx context.Context, month time.Month, year int) error
	}

	DatabaseImpl struct {
		shifts      *mongo.Collection
		rosters     *mongo.Collection
		offTime     *mongo.Collection
		constraints *mongo.Collection
		worktime    *mongo.Collection
		logger      hclog.Logger
		debug       bool
	}
)

func NewDatabase(ctx context.Context, db *mongo.Database, logger hclog.Logger) (*DatabaseImpl, error) {
	impl := &DatabaseImpl{
		shifts:      db.Collection(ShiftCollection),
		rosters:     db.Collection(RosterCollection),
		offTime:     db.Collection(OffTimeRequestCollection),
		constraints: db.Collection(ConstraintCollection),
		worktime:    db.Collection(WorktimeCollection),
		logger:      logger,
		debug:       false,
	}

	if err := impl.setup(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup database: %w", err)
	}

	return impl, nil
}

func (db *DatabaseImpl) SetDebug(v bool) {
	db.debug = v
}

func (db *DatabaseImpl) dumpFilter(msg string, filter any) {
	if db.debug {
		blob, err := json.MarshalIndent(filter, "", "  ")
		if err != nil {
			db.logger.Warn("failed to marshal filter", "error", err.Error())

			return
		}

		db.logger.Info(msg, "filter", string(blob))
	}
}

func (db *DatabaseImpl) setup(ctx context.Context) error {
	db.logger.Debug("creating shift indexes")
	_, err := db.shifts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "to", Value: 1},
				{Key: "days", Value: 1},
				{Key: "onHoliday", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "days", Value: 1},
				{Key: "onHoliday", Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create shift indexes: %w", err)
	}

	_, err = db.rosters.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "year", Value: 1},
				{Key: "month", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create roster indexes: %w", err)
	}

	_, err = db.offTime.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "to", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "staffID", Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create offtime indexes: %w", err)
	}

	return nil
}

// Interfaces check
var _ interface {
	WorkShiftDatabase
	OffTimeDatabase
	ConstraintDatabase
	WorkTimeDatabase
	RosterDatabase
} = new(DatabaseImpl)
