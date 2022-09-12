package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/tierklinik-dobersberg/rosterd/structs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	ShiftCollection          = "rosterd-shifts"
	RosterCollection         = "rosterd-rosters"
	OffTimeRequestCollection = "rosterd-offtime"
)

type (
	WorkShiftDatabase interface {
		SaveWorkShift(context.Context, *structs.WorkShift) error
		DeleteWorkShift(context.Context, string) error
		FindWorkShiftsForDay(context.Context, time.Weekday, bool) ([]structs.WorkShift, error)
		ListWorkShifts(context.Context) ([]structs.WorkShift, error)
		GetShiftsForDay(ctx context.Context, weekDay time.Weekday, isHoliday bool) ([]structs.WorkShift, error)
	}

	OffTimeDatabase interface {
		GetOffTimeRequest(ctx context.Context, id string) (*structs.OffTimeRequest, error)
		CreateOffTimeRequest(ctx context.Context, req *structs.OffTimeRequest) error
		DeleteOffTimeRequest(ctx context.Context, id string) error
		FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string) ([]structs.OffTimeRequest, error)
		UpdateOffTimeRequest(ctx context.Context, upd structs.OffTimeRequestUpdate) error
		ApproveOffTimeRequest(ctx context.Context, id string, approved bool) error
	}

	DatabaseImpl struct {
		shifts  *mongo.Collection
		rosters *mongo.Collection
		offTime *mongo.Collection
		logger  hclog.Logger
	}
)

func NewDatabase(ctx context.Context, db *mongo.Database, logger hclog.Logger) (*DatabaseImpl, error) {
	impl := &DatabaseImpl{
		shifts:  db.Collection(ShiftCollection),
		rosters: db.Collection(RosterCollection),
		offTime: db.Collection(OffTimeRequestCollection),
		logger:  logger,
	}

	if err := impl.setup(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup database: %w", err)
	}

	return impl, nil
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
				{Key: "shiftID", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "to", Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create roster indexes: %w", err)
	}

	return nil
}

// Interfaces check
var (
	_ WorkShiftDatabase = new(DatabaseImpl)
	_ OffTimeDatabase   = new(DatabaseImpl)
)
