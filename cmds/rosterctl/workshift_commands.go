package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func WorkShiftCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "work-shift",
		Aliases: []string{"shift", "workshift"},
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.WorkShift().ListWorkShifts(context.Background(), connect.NewRequest(&rosterv1.ListWorkShiftsRequest{}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	cmd.AddCommand(
		CreateWorkShiftCommand(root),
		DeleteWorkShiftCommand(root),
		UpdateWorkShiftCommand(root),
	)

	return cmd
}

func DeleteWorkShiftCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "delete",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.WorkShift().DeleteWorkShift(context.Background(), connect.NewRequest(&rosterv1.DeleteWorkShiftRequest{
				Id: args[0],
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}

func CreateWorkShiftCommand(root *cli.Root) *cobra.Command {
	var (
		from          string
		duration      time.Duration
		days          []string
		name          string
		displayName   string
		onHoliday     bool
		roles         []string
		worth         int
		requiredCount int
		color         string
		description   string
		order         int
		tags          []string
	)

	cmd := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			parts := strings.SplitN(from, ":", 2)
			if len(parts) != 2 {
				logrus.Fatal("invalid value for --from. Expected format is HH:MM")
			}

			hour, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "0"), 10, 0)
			if err != nil {
				logrus.Fatalf("invalid value for --from: hour: %s", err)
			}

			mins, err := strconv.ParseInt(strings.TrimPrefix(parts[1], "0"), 10, 0)
			if err != nil {
				logrus.Fatalf("invalid value for --from: minutes: %s", err)
			}

			parsedDays, ok := parseDays(days)
			if !ok {
				logrus.Fatalf("invalid value for --days")
			}

			req := &rosterv1.CreateWorkShiftRequest{
				From: &rosterv1.Daytime{
					Hour:   hour,
					Minute: mins,
				},
				Duration:           durationpb.New(duration),
				Days:               parsedDays,
				Name:               name,
				DisplayName:        displayName,
				OnHoliday:          onHoliday,
				EligibleRoleIds:    roles,
				RequiredStaffCount: int64(requiredCount),
				Color:              color,
				Description:        description,
				Order:              int64(order),
				Tags:               tags,
			}

			if cmd.Flag("worth").Changed {
				req.TimeWorth = durationpb.New(time.Duration(worth) * time.Minute)
			}

			res, err := root.WorkShift().CreateWorkShift(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&from, "from", "", "")
		f.DurationVar(&duration, "duration", 0, "")
		f.StringSliceVar(&days, "days", nil, "")
		f.StringVar(&name, "name", "", "")
		f.StringVar(&displayName, "display-name", "", "")
		f.BoolVar(&onHoliday, "holiday", false, "")
		f.StringSliceVar(&roles, "roles", nil, "2")
		f.IntVar(&worth, "worth", 0, "")
		f.IntVar(&requiredCount, "count", 0, "")
		f.StringVar(&color, "color", "", "")
		f.StringVar(&description, "description", "", "")
		f.IntVar(&order, "order", 0, "")
		f.StringSliceVar(&tags, "tag", nil, "")
	}

	return cmd
}

func UpdateWorkShiftCommand(root *cli.Root) *cobra.Command {
	var (
		replaceWorkShift bool
		from             string
		duration         time.Duration
		days             []string
		name             string
		displayName      string
		onHoliday        bool
		roles            []string
		worth            int
		requiredCount    int
		color            string
		description      string
		order            int
		tags             []string
		deleteTimeWorth  bool
	)

	cmd := &cobra.Command{
		Use:  "update",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var dtFrom *rosterv1.Daytime
			if cmd.Flag("from").Changed {
				parts := strings.SplitN(from, ":", 2)
				if len(parts) != 2 {
					logrus.Fatal("invalid value for --from. Expected format is HH:MM")
				}

				hour, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "0"), 10, 0)
				if err != nil {
					logrus.Fatalf("invalid value for --from: hour: %s", err)
				}

				mins, err := strconv.ParseInt(strings.TrimPrefix(parts[1], "0"), 10, 0)
				if err != nil {
					logrus.Fatalf("invalid value for --from: minutes: %s", err)
				}

				dtFrom = &rosterv1.Daytime{
					Hour:   hour,
					Minute: mins,
				}
			}

			var parsedDays []int32
			if cmd.Flag("days").Changed {
				var ok bool
				parsedDays, ok = parseDays(days)
				if !ok {
					logrus.Fatalf("invalid value for --days")
				}
			}

			req := &rosterv1.UpdateWorkShiftRequest{
				Id: args[0],
				Update: &rosterv1.WorkShiftUpdate{
					From:               dtFrom,
					Duration:           durationpb.New(duration),
					Days:               parsedDays,
					Name:               name,
					DisplayName:        displayName,
					OnHoliday:          onHoliday,
					EligibleRoleIds:    roles,
					RequiredStaffCount: int64(requiredCount),
					Color:              color,
					Description:        description,
					Order:              int64(order),
					Tags:               tags,
				},
				UpdateInPlace: !replaceWorkShift,
				WriteMask:     &fieldmaskpb.FieldMask{Paths: make([]string, 0)},
			}

			if cmd.Flag("worth").Changed {
				req.Update.TimeWorth = durationpb.New(time.Duration(worth) * time.Minute)
			}
			if deleteTimeWorth {
				req.Update.TimeWorth = nil
			}

			updateSet := [][]string{
				{"from", "from"},
				{"duration", "duration"},
				{"days", "days"},
				{"display-name", "display_name"},
				{"holiday", "on_holiday"},
				{"name", "name"},
				{"roles", "eligible_role_ids"},
				{"worth", "time_worth"},
				{"delete-worth", "time_worth"},
				{"color", "color"},
				{"count", "required_staff_count"},
				{"description", "description"},
				{"order", "order"},
				{"tag", "tags"},
			}
			for _, s := range updateSet {
				if cmd.Flag(s[0]).Changed {
					req.WriteMask.Paths = append(req.WriteMask.Paths, s[1])
				}
			}

			res, err := root.WorkShift().UpdateWorkShift(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.BoolVar(&deleteTimeWorth, "delete-worth", false, "")
		f.BoolVar(&replaceWorkShift, "replace", true, "")
		f.StringVar(&from, "from", "", "")
		f.DurationVar(&duration, "duration", 0, "")
		f.StringSliceVar(&days, "days", nil, "")
		f.StringVar(&name, "name", "", "")
		f.StringVar(&displayName, "display-name", "", "")
		f.BoolVar(&onHoliday, "holiday", false, "")
		f.StringSliceVar(&roles, "roles", nil, "2")
		f.IntVar(&worth, "worth", 0, "")
		f.IntVar(&requiredCount, "count", 0, "")
		f.StringVar(&color, "color", "", "")
		f.StringVar(&description, "description", "", "")
		f.IntVar(&order, "order", 0, "")
		f.StringSliceVar(&tags, "tag", nil, "")
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

func parseDays(days []string) ([]int32, bool) {
	result := make([]int32, len(days))
	for idx, day := range days {
		parsed, ok := parseDay(day)
		if !ok {
			return nil, false
		}

		result[idx] = int32(parsed)
	}

	return result, true
}
