package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ShiftCollection          = "rosterd-shifts"
	RosterCollection         = "rosterd-rosters"
	OffTimeRequestCollection = "rosterd-offtime"
	OffTimeCostsCollection   = "rosterd-offtime-costs"
	ConstraintCollection     = "rosterd-constraints"
	WorktimeCollection       = "rosterd-worktime"
	DutyRosterCollection     = "rosterd-dutyrosters"
	RosterTypeCollection     = "rosterd-rostertypes"
)

type (
	WorkShiftDatabase interface {
		SaveWorkShift(context.Context, *structs.WorkShift) error
		DeleteWorkShift(context.Context, string) error
		ListWorkShifts(context.Context) ([]structs.WorkShift, error)
		GetShiftsForDay(ctx context.Context, weekDay time.Weekday, isHoliday bool) ([]structs.WorkShift, error)
	}

	OffTimeDatabase interface {
		GetOffTimeRequest(ctx context.Context, ids ...string) ([]structs.OffTimeEntry, error)
		CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeEntry) error
		DeleteOffTimeRequest(ctx context.Context, id ...string) error
		FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, userIds []string) ([]structs.OffTimeEntry, error)
		ApproveOffTimeRequest(ctx context.Context, id string, approval *structs.Approval) error
		AddOffTimeCost(ctx context.Context, cost *structs.OffTimeCosts) error
		GetOffTimeCosts(ctx context.Context, user_ids ...string) ([]structs.OffTimeCosts, error)
		DeleteOffTimeCosts(ctx context.Context, ids ...string) error
		// CalculateOffTimeCredits(ctx context.Context) (map[string]time.Duration, error)
	}

	ConstraintDatabase interface {
		CreateConstraint(ctx context.Context, req *structs.Constraint) error
		UpdateConstraint(ctx context.Context, constraint *structs.Constraint) error
		DeleteConstraint(ctx context.Context, id string) error
		FindConstraints(ctx context.Context, staff []string, roleIds []string) ([]structs.Constraint, error)
	}

	WorkTimeDatabase interface {
		SaveWorkTimePerWeek(ctx context.Context, wt *structs.WorkTime) error
		WorkTimeHistoryForStaff(ctx context.Context, staff string) ([]structs.WorkTime, error)
		GetCurrentWorkTimes(ctx context.Context, until time.Time) (map[string]structs.WorkTime, error)
		DeleteWorkTime(ctx context.Context, ids ...string) error
		GetWorktimeByID(ctx context.Context, id string) (*structs.WorkTime, error)
		UpdateWorkTime(ctx context.Context, wt *structs.WorkTime) error
	}

	DutyRosterDatabase interface {
		SaveDutyRoster(ctx context.Context, roster *structs.DutyRoster, casIndex *uint64) (bool, error)
		DeleteDutyRoster(ctx context.Context, rosterID string, supersededBy primitive.ObjectID) error
		ApproveDutyRoster(ctx context.Context, rosterID, approver string) error
		DutyRosterByID(ctx context.Context, id string) (structs.DutyRoster, error)
		DutyRostersByTime(ctx context.Context, time time.Time) ([]structs.DutyRoster, error)
		GetSupersededDutyRoster(ctx context.Context, rosterID primitive.ObjectID) (*structs.DutyRoster, error)
	}

	DatabaseImpl struct {
		shifts          *mongo.Collection
		rosters         *mongo.Collection
		offTime         *mongo.Collection
		offTimeCosts    *mongo.Collection
		constraints     *mongo.Collection
		worktime        *mongo.Collection
		dutyRosters     *mongo.Collection
		dutyRosterTypes *mongo.Collection
		logger          *logrus.Entry
		debug           bool
	}
)

func NewDatabase(ctx context.Context, db *mongo.Database, logger *logrus.Entry) (*DatabaseImpl, error) {
	impl := &DatabaseImpl{
		shifts:          db.Collection(ShiftCollection),
		rosters:         db.Collection(RosterCollection),
		offTime:         db.Collection(OffTimeRequestCollection),
		offTimeCosts:    db.Collection(OffTimeCostsCollection),
		constraints:     db.Collection(ConstraintCollection),
		worktime:        db.Collection(WorktimeCollection),
		dutyRosters:     db.Collection(DutyRosterCollection),
		dutyRosterTypes: db.Collection(RosterTypeCollection),
		logger:          logger,
		debug:           false,
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
				{Key: "requestorId", Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create offtime indexes: %w", err)
	}

	_, err = db.dutyRosterTypes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "unique_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create roster-type indexes: %w", err)
	}

	return nil
}

// Interfaces check
var _ interface {
	WorkShiftDatabase
	OffTimeDatabase
	ConstraintDatabase
	WorkTimeDatabase
	DutyRosterDatabase
} = new(DatabaseImpl)
