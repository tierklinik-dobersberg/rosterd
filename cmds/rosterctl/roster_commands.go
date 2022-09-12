package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
)

func getRosterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roster [command]",
		Short: "Manage duty rosters",
	}

	cmd.AddCommand(
		getRosterShiftsCommand(),
	)

	return cmd
}

func getRosterShiftsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shifts ( [month] | [from] [to] )",
		Args:  cobra.MinimumNArgs(1),
		Short: "Get an empty duty roster for period of time",
		Run: func(cmd *cobra.Command, args []string) {
			var (
				from time.Time
				to   time.Time
			)

			if len(args) == 1 {
				monthIdx, err := strconv.ParseInt(args[0], 0, 0)
				if err != nil {
					hclog.L().Error("failed to parse month", "error", err)
					os.Exit(1)
				}

				now := time.Now()

				from = time.Date(now.Year(), time.Month(monthIdx), 1, 0, 0, 0, 0, time.Local)
				to = time.Date(now.Year(), time.Month(monthIdx)+1, 0, 0, 0, 0, 0, time.Local)
			} else {
				var err error
				from, err = time.Parse("2006-01-02", args[0])
				if err != nil {
					hclog.L().Error("failed to parse from time", "error", err)
					os.Exit(1)
				}

				to, err = time.Parse("2006-01-02", args[1])
				if err != nil {
					hclog.L().Error("failed to parse to time", "error", err)
					os.Exit(1)
				}
			}

			result, err := cli.GetRequiredShifts(cmd.Context(), from, to)
			if err != nil {
				hclog.L().Error("failed to retrieve empty roster", "error", err, "from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"))
				os.Exit(1)
			}

			dayHeader := color.New(color.Bold, color.Underline).Sprint
			shiftName := color.New(color.FgGreen, color.Bold).Sprint
			timeRange := color.New(color.Italic).Sprintf

			var keys = make([]string, 0, len(result))
			for key := range result {
				keys = append(keys, key)
			}

			sort.Strings(keys)

			for _, key := range keys {
				shifts := result[key]
				d, _ := time.Parse("2006-01-02", key)

				fmt.Println(dayHeader(d.Format("2006-01-02")+":") + " " + timeRange(d.Format("(Monday)")))
				for _, shift := range shifts {

					format := "15:04"

					tr := timeRange("%s - %s", shift.From.Format(format), shift.To.Format(format))
					if shift.From.YearDay() != shift.To.YearDay() {
						tr = timeRange("%s - %s", shift.From.Format(format), shift.To.Format("15:04 (2006-01-02)"))
					}

					fmt.Println(" • " + shiftName(shift.Name) + ": " + tr)
				}
				fmt.Println()
			}
		},
	}

	return cmd
}
