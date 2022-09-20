package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func getRosterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roster [command]",
		Short: "Manage duty rosters",
	}

	cmd.AddCommand(
		getRosterShiftsCommand(),
		getGenerateRosterCommand(),
	)

	return cmd
}

func getRosterShiftsCommand() *cobra.Command {
	var (
		evalConstraints bool
		detailed        bool
	)

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

			result, err := cli.GetRequiredShifts(cmd.Context(), from, to, evalConstraints)
			if err != nil {
				hclog.L().Error("failed to retrieve empty roster", "error", err, "from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"))
				os.Exit(1)
			}

			dayHeader := text.Colors{text.Bold, text.Underline}.Sprint
			shiftName := text.Colors{text.FgGreen, text.Bold}.Sprint
			timeRange := text.Colors{text.Italic}.Sprintf

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

					if detailed {
						fmt.Println("     Worth: " + (time.Duration(shift.MinutesWorth) * time.Minute).String())
					}

					if evalConstraints {
						fmt.Println("       Staff: " + timeRange(strings.Join(shift.EligibleStaff, ", ")))

						if detailed {
							if len(shift.Violations) > 0 {
								fmt.Println("       Constraints:")
								for user, violations := range shift.Violations {
									fmt.Println("       • " + user)
									for _, v := range violations {
										fmt.Println("           • " + v.Type + ": " + v.Name)
									}
								}
							}
						}
					}
				}
				fmt.Println()
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.BoolVar(&evalConstraints, "eval", false, "Evaluate constraints and include possible staff")
		flags.BoolVar(&detailed, "detail", false, "Show detailed information. Only applicable with --eval")
	}

	return cmd
}

func getGenerateRosterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "generate [year] [month]",
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			year, err := strconv.ParseInt(args[0], 0, 0)
			if err != nil {
				hclog.L().Error("failed to parse year", "year", args[0])
				os.Exit(1)
			}

			month, err := strconv.ParseInt(args[1], 0, 0)
			if err != nil {
				hclog.L().Error("failed to parse month", "month", args[1])
				os.Exit(1)
			}

			res, err := cli.GenerateRoster(cmd.Context(), int(year), time.Month(month))
			if err != nil {
				hclog.L().Error("failed to generate roster", "error", err)
				os.Exit(1)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "    ")

			enc.Encode(res)
		},
	}

	return cmd
}
