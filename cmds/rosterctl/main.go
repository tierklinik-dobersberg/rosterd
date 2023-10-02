package main

import (
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"github.com/tierklinik-dobersberg/rosterd/cmds/rosterctl/cmds"
)

func getRootCommand(root *cli.Root) {
	root.BaseURLS = cli.BaseURLS{
		Idm:      "https://account.dobersberg.dev",
		Calendar: "https://calendar.dobersberg.dev",
		Roster:   "https://roster.dobersberg.dev",
	}

	root.AddCommand(
		cmds.WorkTimeCommand(root),
		cmds.OffTimeCommand(root),
		cmds.WorkShiftCommand(root),
		cmds.RosterCommand(root),
		cmds.ConstraintCommand(root),
	)
}

func main() {
	cmd := cli.New("rosterctl")

	getRootCommand(cmd)

	if err := cmd.Execute(); err != nil {
		logrus.Fatal(err.Error())
	}
}
