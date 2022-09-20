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

func getOffTimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "offtime [command]",
		Aliases: []string{"vacation", "off", "vac"},
		Short:   "Manage off-time requests",
	}

	cmd.AddCommand(
		getCreateOffTimeRequestCommand(),
		getListOffTimeRequestsCommand(),
		getApproveOffTimeRequestsCommand(),
		getRejectOffTimeRequestsCommand(),
		getDeleteOffTimeRequestsCommand(),
	)

	return cmd
}

func getCreateOffTimeRequestCommand() *cobra.Command {
	var (
		req structs.OffTimeRequest
	)

	cmd := &cobra.Command{
		Use:     "create [from] [to]",
		Aliases: []string{"request", "req"},
		Short:   "Create a new off-time request",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			from, err := time.Parse("2006-01-02", args[0])
			if err != nil {
				hclog.L().Error("invalid from time", "error", err)
				os.Exit(1)
			}

			to, err := time.Parse("2006-01-02", args[1])
			if err != nil {
				hclog.L().Error("invalid to time", "error", err)
				os.Exit(1)
			}

			req.From = from
			req.To = to

			if err := cli.CreateOffTimeRequest(cmd.Context(), req); err != nil {
				hclog.L().Error("failed to create off-time request", "error", err)
				os.Exit(1)
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.StringVar(&req.StaffID, "staff", "", "The name of the staff")
		flags.BoolVar(&req.IsSoftConstraint, "soft", false, "This is a soft request")
		flags.StringVar(&req.Description, "reason", "", "A descriptive reason for the off-time request")
	}

	return cmd
}

func getListOffTimeRequestsCommand() *cobra.Command {
	var (
		from       string
		to         string
		approved   bool
		staff      []string
		long       bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List off-time requests",
		Run: func(cmd *cobra.Command, args []string) {
			var (
				fromTime       time.Time
				toTime         time.Time
				approvedFilter *bool
			)
			if from != "" {
				var err error
				fromTime, err = time.Parse("2006-01-02", from)
				if err != nil {
					hclog.L().Error("invalid from time", "error", err)
					os.Exit(1)
				}
			}

			if to != "" {
				var err error
				toTime, err = time.Parse("2006-01-02", to)
				if err != nil {
					hclog.L().Error("invalid to time", "error", err)
					os.Exit(1)
				}
			}

			if cmd.Flag("approved").Changed {
				approvedFilter = &approved
			}

			res, err := cli.FindOffTimeRequests(cmd.Context(), fromTime, toTime, approvedFilter, staff)
			if err != nil {
				hclog.L().Error("failed to fetch off-time requests", "error", err)
				os.Exit(1)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "    ")
				enc.Encode(res)

				return
			}

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)

			t.SetStyle((table.StyleRounded))
			t.Style().Color.Header = text.Colors{text.FgHiWhite, text.Bold}
			t.Style().Options.DrawBorder = false
			t.Style().Options.SeparateColumns = false
			t.Style().Options.SeparateHeader = false
			t.Style().Options.SeparateRows = false

			t.SetColumnConfigs([]table.ColumnConfig{
				{
					Name: "Approved",
					Transformer: func(val any) string {
						v := val.(string)
						if v == "✓" {
							return text.Colors{text.FgGreen, text.Bold}.Sprint("✓")
						} else if v == "𐄂" {
							return text.Colors{text.FgRed, text.Bold}.Sprint("𐄂")
						}

						return ""
					},
					Align:  text.AlignCenter,
					Hidden: !long,
				},
			})

			t.AppendHeader(table.Row{
				"ID",
				"From",
				"To",
				"Duration",
				"Staff",
				"Approved",
				"Description",
			})

			for _, req := range res {
				var approved string
				if req.Approved != nil {
					if *req.Approved {
						approved = "✓"
					} else {
						approved = "𐄂"
					}
				}
				t.AppendRow(table.Row{
					req.ID.Hex(),
					req.From.Format("2006-01-02"),
					req.To.Format("2006-01-02"),
					req.To.Sub(req.From).String(),
					req.StaffID,
					approved,
					req.Description,
				})
			}

			t.Render()
		},
	}

	flags := cmd.Flags()
	{
		flags.BoolVarP(&long, "long", "l", false, "Display long output")
		flags.BoolVar(&approved, "approved", false, "Only search for approved or rejected requests")
		flags.StringVar(&from, "from", "", "Only search for off-time requests after this date")
		flags.StringVar(&to, "to", "", "Only search for off-time requests before this date")
		flags.StringSliceVar(&staff, "staff", nil, "Only search for off-time requests of the give staff")
		flags.BoolVar(&jsonOutput, "json", false, "Display result in JSON")
	}

	return cmd
}

func getDeleteOffTimeRequestsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete off-time requests",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cli.DeleteOffTimeRequest(cmd.Context(), args[0]); err != nil {
				hclog.L().Error("failed to approved off-time request", "error", err)
				os.Exit(1)
			}
		},
	}

	return cmd
}

func getApproveOffTimeRequestsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve [id]",
		Short: "Approve off-time requests",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cli.ApproveOffTimeRequest(cmd.Context(), args[0], true); err != nil {
				hclog.L().Error("failed to approved off-time request", "error", err)
				os.Exit(1)
			}
		},
	}

	return cmd
}

func getRejectOffTimeRequestsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject [id]",
		Short: "Reject off-time requests",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cli.ApproveOffTimeRequest(cmd.Context(), args[0], false); err != nil {
				hclog.L().Error("failed to reject off-time request", "error", err)
				os.Exit(1)
			}
		},
	}

	return cmd
}
