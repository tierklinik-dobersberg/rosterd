package main

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
)

func getRootCommand(root *cli.Root) {
	root.BaseURLS = cli.BaseURLS{
		Idm:      "https://account.dobersberg.dev",
		Calendar: "https://calendar.dobersberg.dev",
		Roster:   "https://roster.dobersberg.dev",
	}

	root.AddCommand(
		WorkTimeCommand(root),
		OffTimeCommand(root),
		WorkShiftCommand(root),
		RosterCommand(root),
		ConstraintCommand(root),
	)
}

func main() {
	cmd := cli.New("rosterctl")

	getRootCommand(cmd)

	if err := cmd.Execute(); err != nil {
		logrus.Fatal(err.Error())
	}
}

func getUserMap(root *cli.Root) map[string]*idmv1.Profile {
	res, err := root.Users().ListUsers(context.Background(), connect.NewRequest(&idmv1.ListUsersRequest{}))

	if err != nil {
		logrus.Fatalf("failed to fetch users: %s", err)
	}

	m := make(map[string]*idmv1.Profile)

	for _, u := range res.Msg.Users {
		m[u.User.Id] = u
	}

	return m
}
