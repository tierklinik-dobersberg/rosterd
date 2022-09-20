package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/cis/pkg/daytime"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func getWorkShiftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workshift [command]",
		Short: "Manage workshift definitions",
	}

	cmd.AddCommand(
		getCreateWorkshiftCommand(),
		getListWorkshiftCommand(),
		getDeleteWorkShiftCommand(),
	)

	return cmd
}

func getListWorkshiftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List and search for working shifts",
		Run: func(cmd *cobra.Command, args []string) {
			shifts, err := cli.ListWorkShifts(cmd.Context())
			if err != nil {
				hclog.L().Error("failed to retrieve work shift list", "error", err)
				os.Exit(1)
			}

			tb := getTbWriter()
			tb.AppendHeader(table.Row{
				"Name",
				"From",
				"Duration",
				"Worth",
				"Count",
				"Days",
				"Holiday",
				"ID",
			})

			for _, s := range shifts {
				tb.AppendRow(table.Row{
					s.Name,
					s.From,
					s.Duration,
					s.MinutesWorth,
					s.RequiredStaffCount,
					s.Days,
					s.OnHoliday,
					s.ID.Hex(),
				})
			}

			tb.Render()
		},
	}

	return cmd
}

func getDeleteWorkShiftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.MinimumNArgs(1),
		Use:   "delete <id>",
		Short: "Delete a workshift definition by id",
		Run: func(cmd *cobra.Command, args []string) {
			hasError := false
			for _, id := range args {
				err := cli.DeleteWorkShift(cmd.Context(), id)
				if err != nil {
					hclog.L().Error("failed to delete work shift", "id", id, "error", err)
					hasError = true
				}
			}
			if hasError {
				os.Exit(1)
			}
		},
	}

	return cmd
}

func getCreateWorkshiftCommand() *cobra.Command {
	var workShift structs.WorkShift

	var (
		from         string
		to           time.Duration
		days         []string
		minutesWorth int
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new workshift",
		Run: func(cmd *cobra.Command, args []string) {
			// get a list of week days for this shift
			for _, day := range days {
				d, ok := parseDay(day)
				if !ok {
					hclog.L().Error("invalid day", "day", day)
					os.Exit(1)
				}
				workShift.Days = append(workShift.Days, d)
			}

			workShift.Duration = to

			fromTime, err := daytime.ParseDayTime(from)
			if err != nil {
				hclog.L().Error("invalid 'from' time", "error", err)
				os.Exit(1)
			}
			workShift.From = structs.Daytime(fromTime.AsDuration())

			if minutesWorth != 0 {
				workShift.MinutesWorth = &minutesWorth
			}

			if err := cli.CreateWorkShift(context.Background(), workShift); err != nil {
				hclog.L().Error("failed to create work shift", "error", err)
				os.Exit(1)
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.StringSliceVarP(&workShift.EligibleRoles, "roles", "r", nil, "List of roles eligible for this workshift")
		flags.BoolVar(&workShift.OnHoliday, "holiday", false, "Valid on holidays")
		flags.StringVarP(&from, "from", "f", "", "Start time")
		flags.DurationVarP(&to, "duration", "D", 0, "Duration of the work shift")
		flags.StringSliceVarP(&days, "days", "d", nil, "A list of weekdays for this work shift")
		flags.IntVarP(&minutesWorth, "worth", "w", 0, "How many minutes this work shift is worth")
		flags.IntVarP(&workShift.RequiredStaffCount, "staff-count", "c", 0, "Number of employees required for this shift")
		flags.StringVarP(&workShift.Name, "name", "n", "", "Descriptive name for this work shift")
	}

	return cmd
}

// parseDay parses the weekday specified in day.
func parseDay(day string) (time.Weekday, bool) {
	days := map[string]time.Weekday{
		"mo": time.Monday,
		"tu": time.Tuesday,
		"we": time.Wednesday,
		"th": time.Thursday,
		"fr": time.Friday,
		"sa": time.Saturday,
		"su": time.Sunday,
	}

	if len(day) < 2 {
		return 0, false
	}

	d, ok := days[strings.ToLower(day[0:2])]

	return d, ok
}
