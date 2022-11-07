package e2e_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/tierklinik-dobersberg/rosterd/e2e/framework"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type offTimeTestSuite struct {
	ctx context.Context

	suite.Suite
	*framework.Environment
}

func newOffTimeSuite(ctx context.Context, env *framework.Environment) *offTimeTestSuite {
	return &offTimeTestSuite{
		Environment: env,
		ctx:         ctx,
	}
}

func (ot *offTimeTestSuite) SetupSuite() {
	admin := ot.Identitiy.User("admin", "admin")

	startOfWork := time.Date(2022, 01, 01, 0, 0, 0, 0, time.UTC)

	err := admin.SetWorkTime(ot.ctx, structs.WorkTime{
		Staff:                 "admin",
		TimePerWeek:           time.Hour*38 + 30*time.Minute,
		ApplicableFrom:        startOfWork,
		VacationAutoGrantDays: 0,
		EmploymentStart:       startOfWork,
	})

	ot.Require().NoError(err)

	err = admin.SetWorkTime(ot.ctx, structs.WorkTime{
		Staff:                 "user",
		TimePerWeek:           time.Hour * 30,
		ApplicableFrom:        startOfWork,
		VacationAutoGrantDays: 0,
		EmploymentStart:       startOfWork,
	})

	ot.Require().NoError(err)
}

func (ot *offTimeTestSuite) findRequest(id string, sl []structs.OffTimeEntry) *structs.OffTimeEntry {
	for _, e := range sl {
		if e.ID.Hex() == id {
			return &e
		}
	}

	return nil
}

func (ot *offTimeTestSuite) Test_Create_Delete_OffTime_Request() {
	from := time.Now().Add(24 * time.Hour)
	to := time.Now().Add(7 * 24 * time.Hour)

	cli := ot.Identitiy.User("user")
	entry, err := cli.CreateOffTimeRequest(ot.ctx, structs.CreateOffTimeRequest{
		From:        from,
		To:          to,
		RequestType: structs.RequestTypeAuto,
	})

	ot.Assert().NoError(err)
	ot.Require().NotNil(entry)

	requests, err := cli.FindOffTimeRequests(ot.ctx, time.Time{}, time.Time{}, nil, nil)
	ot.Require().NoError(err)

	req := ot.findRequest(entry.ID.Hex(), requests)
	ot.Require().NotNil(req)
	ot.Assert().Equal(entry, req)
}
