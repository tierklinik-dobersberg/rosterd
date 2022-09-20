package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func getWorkTimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktime",
		Short: "Manage work-time per week for employees",
		Run: func(cmd *cobra.Command, args []string) {
			res, err := cli.GetCurrentWorkTimes(cmd.Context())
			if err != nil {
				hclog.L().Error("failed to retrieve current work times", "error", err)
				os.Exit(1)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "    ")
			enc.Encode(res)
		},
	}

	cmd.AddCommand(
		getSetWorkTimeCommand(),
		getGetWorkTimeHistoryCommand(),
	)

	return cmd
}

func getSetWorkTimeCommand() *cobra.Command {
	var validFrom string
	var overtimePenalty float64
	var undertimePenalty float64

	cmd := &cobra.Command{
		Use:   "set [staff] [timePerWeek]",
		Short: "Set the amount of time an employee is expected to work per week",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			staff := args[0]
			timePerWeek, err := time.ParseDuration(args[1])
			if err != nil {
				hclog.L().Error("failed to parse time-per-week", "error", err)
				os.Exit(1)
			}

			applicableFrom, err := time.Parse("2006-01-02", validFrom)
			if err != nil {
				hclog.L().Error("failed to parse --start-with", "error", err)
				os.Exit(1)
			}

			req := structs.WorkTime{
				Staff:                 staff,
				TimePerWeek:           timePerWeek,
				ApplicableFrom:        applicableFrom,
				OvertimePenaltyRatio:  overtimePenalty,
				UndertimePenaltyRatio: undertimePenalty,
			}

			if err := cli.SetWorkTime(cmd.Context(), req); err != nil {
				hclog.L().Error("failed to set worktime", "error", err)
				os.Exit(1)
			}
		},
	}

	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)

	flags := cmd.Flags()
	{
		flags.StringVarP(&validFrom, "start-with", "S", tomorrow.Format("2006-01-02"), "Date at which the new work-time is applicable, defaults to tomorrow")
		flags.Float64Var(&overtimePenalty, "overtime-penalty", 0, "")
		flags.Float64Var(&undertimePenalty, "undertime-penalty", 0, "")
	}

	return cmd
}

func getTbWriter() table.Writer {

	tb := table.NewWriter()
	tb.SetOutputMirror(os.Stdout)

	tb.SetStyle((table.StyleRounded))
	tb.Style().Color.Header = text.Colors{text.FgHiWhite, text.Bold}
	tb.Style().Options.DrawBorder = false
	tb.Style().Options.SeparateColumns = false
	tb.Style().Options.SeparateHeader = false
	tb.Style().Options.SeparateRows = false

	return tb
}

func getGetWorkTimeHistoryCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "history [staff]",
		Short: "Show history of expected work-time per week for a staff member",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := cli.GetWorkTimeHistory(cmd.Context(), args[0])
			if err != nil {
				hclog.L().Error("failed to get worktime history", "staff", args[0], "error", err)
				os.Exit(1)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "    ")
				enc.Encode(res)

				return
			}

			tb := getTbWriter()
			tb.AppendHeader(table.Row{
				"Staff",
				"Time Per Week",
				"Applicable From",
			})

			for _, entry := range res {
				tb.AppendRow(table.Row{
					entry.Staff,
					entry.TimePerWeek.String(),
					entry.ApplicableFrom,
				})
			}

			tb.Render()
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Display result in JSON")

	return cmd
}
