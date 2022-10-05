package e2e_test

import (
	"context"
	"os"
	"os/signal"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/tierklinik-dobersberg/rosterd/e2e/framework"
)

func Test_RunE2E(t *testing.T) {
	ctx := context.Background()

	ch := make(chan os.Signal, 1)
	go signal.Notify(ch, os.Interrupt)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ch
		cancel()
	}()

	env, err := framework.SetupEnvironment(ctx, t)
	if err != nil {
		t.Errorf("failed to setup environment: %s", err)
		t.FailNow()
	}
	defer env.Teardown(ctx)

	RunTests(ctx, t, env)
}

func RunTests(ctx context.Context, t *testing.T, env *framework.Environment) {
	suite.Run(t, newOffTimeSuite(ctx, env))
}
