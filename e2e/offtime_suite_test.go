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

func (ot *offTimeTestSuite) Test_Create_OffTime_Request() {
	from := time.Now().Add(24 * time.Hour)
	to := time.Now().Add(7 * 24 * time.Hour)

	cli := ot.Identitiy.GetClient("user")
	err := cli.CreateOffTimeRequest(ot.ctx, structs.CreateOffTimeRequest{
		From:        from,
		To:          to,
		RequestType: structs.RequestTypeAuto,
	})

	ot.Assert().NoError(err)

	req, err := cli.FindOffTimeRequests(ot.ctx, time.Now(), time.Time{}, nil, nil)
	ot.Require().NoError(err)

	ot.Assert().Len(req, 1)
}
