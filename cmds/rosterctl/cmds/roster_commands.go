package cmds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func RosterCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use: "roster",
	}

	cmd.AddCommand(
		AnalyzeWorkTimeCommand(root),
		RequiredShiftsCommmand(root),
		WorkingStaffCommand(root),
		RosterTypeCommand(root),
	)

	return cmd
}

func DeleteRosterTypeCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		Run: func(cmd *cobra.Command, args []string) {
			_, err := root.Roster().DeleteRosterType(context.Background(), connect.NewRequest(&rosterv1.DeleteRosterTypeRequest{
				UniqueName: args[0],
			}))
			if err != nil {
				logrus.Fatal(err)
			}
		},
	}

	return cmd
}

func CreateRosterTypeCommand(root *cli.Root) *cobra.Command {
	var (
		shiftTags  []string
		onCallTags []string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"save", "update"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.Roster().CreateRosterType(context.Background(), connect.NewRequest(&rosterv1.CreateRosterTypeRequest{
				RosterType: &rosterv1.RosterType{
					UniqueName: args[0],
					ShiftTags:  shiftTags,
					OnCallTags: onCallTags,
				},
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	cmd.Flags().StringSliceVar(&shiftTags, "shift-tag", nil, "A list of shift tags for this roster type")
	cmd.Flags().StringSliceVar(&onCallTags, "on-call-tag", nil, "A list of on-call shift tags for this roster type")

	return cmd
}

func RosterTypeCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "types",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.Roster().ListRosterTypes(context.Background(), connect.NewRequest(&rosterv1.ListRosterTypesRequest{}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	cmd.AddCommand(
		CreateRosterTypeCommand(root),
		DeleteRosterTypeCommand(root),
	)

	return cmd
}

func AnalyzeWorkTimeCommand(root *cli.Root) *cobra.Command {
	var (
		from   string
		to     string
		users  []string
		fields []string
		pretty bool
	)

	cmd := &cobra.Command{
		Use: "work-times",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.AnalyzeWorkTimeRequest{
				From: from,
				To:   to,
				Users: &rosterv1.UsersToAnalyze{
					UserIds:  users,
					AllUsers: len(users) == 0,
				},
				// ReadMask: fieldmaskpb.FieldMask{
				// 	Paths: fields,
				// },
			}

			res, err := root.Roster().AnalyzeWorkTime(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			if !pretty {
				root.Print(res.Msg)
				return
			}

			users := getUserMap(root)

			for _, user := range res.Msg.Results {
				displayName := users[user.UserId].User.DisplayName
				if displayName == "" {
					displayName = users[user.UserId].User.Username
				}

				timeResultColor := text.BgYellow
				timeResultKey := "Overtime: "
				if user.ExpectedTime.AsDuration() > user.PlannedTime.AsDuration() {
					timeResultColor = text.BgRed
					timeResultKey = "Undertime: "
				}

				fmt.Printf(
					"User %s\nID: %s\nExpected: %s\nPlanned: %s\n%s\n\n%s\n",
					text.Colors{text.Bold, text.Underline, text.FgGreen}.Sprint(displayName),
					user.UserId,
					text.Colors{text.Bold, text.Underline, text.FgWhite}.Sprint(shortDur(user.ExpectedTime)),
					text.Colors{text.Bold, text.Underline, text.FgWhite}.Sprint(shortDur(user.PlannedTime)),
					fmt.Sprintf("%s%s", timeResultKey, text.Colors{timeResultColor, text.FgBlack, text.Bold}.Sprintf(" %s ", user.ExpectedTime.AsDuration()-user.PlannedTime.AsDuration())),
					text.Underline.Sprint("Steps:"),
				)

				tbw := getTbWriter()
				tbw.AppendHeader(table.Row{
					"WT",
					"KW",
					"Working Days",
					"Expected Time",
					"Planned",
				})
				tbw.AppendRow(table.Row{"", "", "", "", ""})

				sumDays := 0
				for _, step := range user.Steps {
					idx := shortDur(step.WorkTimePerWeek)

					weekSum := 0
					for _, week := range step.Weeks {
						tbw.AppendRow(table.Row{
							idx,
							fmt.Sprintf("%d KW%02d", week.Year, week.Week),
							week.WorkingDays,
							shortDur(week.ExpectedWork),
							shortDur(week.Planned),
						})
						weekSum += int(week.WorkingDays)
					}

					sumRow := text.Colors{text.FgYellow}.Sprintf
					tbw.AppendRow(table.Row{
						sumRow("Sum"),
						"",
						sumRow("%d", weekSum),
						sumRow(shortDur(step.ExpectedWorkTime)),
						sumRow(shortDur(step.Planned)),
					})

					tbw.AppendRow(table.Row{"", "", "", "", ""})

					sumDays += weekSum
				}

				tbw.AppendRow(table.Row{
					text.Underline.Sprint("Total"),
					"",
					text.Underline.Sprintf("%d", sumDays),
					text.Underline.Sprint(shortDur(user.ExpectedTime)),
					text.Underline.Sprint(shortDur(user.PlannedTime)),
				})

				tbw.Render()
			}
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&from, "from", "", "")
		f.StringVar(&to, "to", "", "")
		f.StringSliceVar(&users, "users", nil, "")
		f.StringSliceVar(&fields, "field", nil, "")
		f.BoolVar(&pretty, "pretty", false, "")
	}

	return cmd
}

func RequiredShiftsCommmand(root *cli.Root) *cobra.Command {
	var (
		from string
		to   string
	)
	cmd := &cobra.Command{
		Use: "plan",
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.Roster().GetRequiredShifts(context.Background(), connect.NewRequest(&rosterv1.GetRequiredShiftsRequest{
				From: from,
				To:   to,
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	f.StringVar(&from, "from", "", "")
	f.StringVar(&to, "to", "", "")

	return cmd
}

func WorkingStaffCommand(root *cli.Root) *cobra.Command {
	var (
		t         string
		typeName  string
		onCall    bool
		shiftTags []string
	)

	cmd := &cobra.Command{
		Use: "working-staff",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.GetWorkingStaffRequest2{
				RosterTypeName: typeName,
				OnCall:         onCall,
				ShiftTags:      shiftTags,
			}

			if t != "" {
				time, err := time.Parse(time.RFC3339, t)
				if err != nil {
					logrus.Fatalf("invalid value for --time")
				}

				req.Query = &rosterv1.GetWorkingStaffRequest2_Time{
					Time: timestamppb.New(time),
				}
			} else {
				req.Query = &rosterv1.GetWorkingStaffRequest2_Time{
					Time: timestamppb.Now(),
				}
			}

			res, err := root.Roster().GetWorkingStaff2(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	cmd.Flags().StringVar(&t, "time", "", "")
	cmd.Flags().StringVar(&typeName, "roster-type", "", "The name of the roster type")
	cmd.Flags().BoolVar(&onCall, "on-call", false, "Only return staff assigned to on-call shifts.")
	cmd.Flags().StringSliceVar(&shiftTags, "tag", nil, "Filter by shift tags")

	return cmd
}

func shortDur(dpb *durationpb.Duration) string {
	d := dpb.AsDuration()
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
