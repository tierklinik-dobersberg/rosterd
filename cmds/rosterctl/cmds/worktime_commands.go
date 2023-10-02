package cmds

import (
	"context"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func WorkTimeCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktime",
		Short: "Manage work-time per week for employees",
	}

	cmd.AddCommand(
		SetWorkTimeCommand(root),
		GetWorkTimesCommand(root),
		GetVacationCreditsLeftCommand(root),
		DeleteWorkTimeCommand(root),
	)

	return cmd
}

func SetWorkTimeCommand(root *cli.Root) *cobra.Command {
	var (
		userId               string
		vacationWeeksPerYear float32
		timePerWeek          time.Duration
		applicableFrom       string
	)

	cmd := &cobra.Command{
		Use: "set",
		Run: func(cmd *cobra.Command, args []string) {
			wt := &rosterv1.WorkTime{
				UserId:               userId,
				TimePerWeek:          durationpb.New(timePerWeek),
				VacationWeeksPerYear: vacationWeeksPerYear,
			}

			if applicableFrom != "" {
				t, err := time.ParseInLocation("2006-01-02", applicableFrom, time.Local)
				if err != nil {
					logrus.Fatalf("failed to parse --from value: %s", err)
				}

				wt.ApplicableAfter = timestamppb.New(t)
			}

			res, err := root.WorkTime().SetWorkTime(context.Background(), connect.NewRequest(&rosterv1.SetWorkTimeRequest{
				WorkTimes: []*rosterv1.WorkTime{wt},
			}))

			if err != nil {
				logrus.Fatalf("failed to set work-time: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&userId, "user-id", "", "The ID of the user")
		f.Float32Var(&vacationWeeksPerYear, "vacation-weeks", 5, "How many weeks of vacation should be granted in a full work-year")
		f.DurationVar(&timePerWeek, "work-time", 40*time.Hour, "How many time the user is expected to work per week")
		f.StringVar(&applicableFrom, "from", "", "After which date (YYYY-MM-DD) this work-time entry is effective.")
	}

	return cmd
}

func GetWorkTimesCommand(root *cli.Root) *cobra.Command {
	var paths []string

	cmd := &cobra.Command{
		Use: "get",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.GetWorkTimeRequest{
				UserIds: args,
			}

			if len(paths) > 0 {
				req.ReadMask = &fieldmaskpb.FieldMask{
					Paths: paths,
				}
			}

			res, err := root.WorkTime().GetWorkTime(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to get work-times: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}

func DeleteWorkTimeCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "delete",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.WorkTime().DeleteWorkTime(context.Background(), connect.NewRequest(&rosterv1.DeleteWorkTimeRequest{
				Ids: args,
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}

func GetVacationCreditsLeftCommand(root *cli.Root) *cobra.Command {
	var (
		until   string
		analyze bool
	)

	cmd := &cobra.Command{
		Use: "vacation-credits",
		Run: func(cmd *cobra.Command, args []string) {

			var untilTime *timestamppb.Timestamp

			if until != "" {
				untilTime = parseFormats(until, "2006-01-02", time.RFC3339)
			}

			res, err := root.WorkTime().GetVacationCreditsLeft(context.Background(), connect.NewRequest(&rosterv1.GetVacationCreditsLeftRequest{
				ForUsers: &rosterv1.SumForUsers{},
				Analyze:  analyze,
				Until:    untilTime,
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			if !analyze {
				root.Print(res.Msg)

				return
			}

			users := getUserMap(root)

			for _, userSum := range res.Msg.Results {
				displayName := users[userSum.UserId].User.DisplayName
				if displayName == "" {
					displayName = users[userSum.UserId].User.Username
				}

				fmt.Printf(
					"User %s\nID: %s\nCredits: %s\n\n%s\n",
					text.Colors{text.Bold, text.Underline, text.FgGreen}.Sprint(displayName),
					userSum.UserId,
					text.Colors{text.Bold, text.Underline, text.FgWhite}.Sprint(userSum.VacationCreditsLeft.AsDuration().Round(time.Hour).String()),
					text.Underline.Sprint("Work Times:"),
				)

				tbl := getTbWriter()
				tbl.AppendHeader(table.Row{
					"From",
					"To",
					"Days",
					"Time/Week",
					"Vacation/Year",
					"Credit-Gain",
					"Weeks",
					"Costs",
					"Credits-Left",
				})

				for _, anal := range userSum.Analysis.Slices {
					perWorkTime := anal.VacationPerWorkTime.AsDuration().Round(time.Hour)
					creditsLeft := perWorkTime + anal.CostsSum.AsDuration()

					tbl.AppendRow(table.Row{
						anal.WorkTime.ApplicableAfter.AsTime().Format("2006-01-02"),
						anal.EndsAt.AsTime().Format("2006-01-02"),
						anal.NumberOfDays,
						anal.WorkTime.TimePerWeek.AsDuration().String(),
						fmt.Sprintf("%.2f Weeks", anal.WorkTime.VacationWeeksPerYear),
						perWorkTime.String(),
						float64(perWorkTime) / float64(anal.WorkTime.TimePerWeek.AsDuration()),
						anal.CostsSum.AsDuration().String(),
						fmt.Sprintf("%s / %.2f Weeks", creditsLeft.String(), float64(creditsLeft)/float64(anal.WorkTime.TimePerWeek.AsDuration())),
					})
				}

				tbl.Render()
			}
		},
	}

	cmd.Flags().StringVar(&until, "until", "", "")
	cmd.Flags().BoolVar(&analyze, "analyze", false, "Display analysis details")

	return cmd
}
