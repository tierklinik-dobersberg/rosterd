package main

import (
	"context"
	"os"
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

func OffTimeCommand(root *cli.Root) *cobra.Command {
	var (
		from    string
		to      string
		userIds []string
		fields  []string
	)

	cmd := &cobra.Command{
		Use:     "offtime [command]",
		Aliases: []string{"vacation", "off", "vac"},
		Short:   "Manage off-time requests",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.FindOffTimeRequestsRequest{
				UserIds: userIds,
				ReadMask: &fieldmaskpb.FieldMask{
					Paths: fields,
				},
			}

			if from != "" {
				req.From = parseFormats(from, "2006-01-02", time.RFC3339)
			}

			if to != "" {
				req.To = parseFormats(to, "2006-01-02", time.RFC3339)
			}

			res, err := root.OffTime().FindOffTimeRequests(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to find off-time request: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&from, "from", "", "")
		f.StringVar(&to, "to", "", "")
		f.StringSliceVar(&userIds, "for-user", nil, "A list of user IDs")
		f.StringSliceVar(&fields, "fields", nil, "Specify a read_mask for the request")
	}

	cmd.AddCommand(
		CreateOffTimeRequestCommand(root),
		ApproveOrRejectCommand(root),
		DeleteOffTimeRequestCommand(root),
		AddOffTimeCostsCommand(root),
		GetOffTimeCostsCommand(root),
		DeleteOffTimeCostsCommand(root),
	)

	return cmd
}

func AddOffTimeCostsCommand(root *cli.Root) *cobra.Command {
	var (
		offtimeId  string
		rosterId   string
		duration   time.Duration
		isVacation bool
		userId     string
		date       string
		credit     bool
	)

	cmd := &cobra.Command{
		Use: "add-costs",
		Run: func(cmd *cobra.Command, args []string) {
			var protoDate *timestamppb.Timestamp

			if date != "" {
				protoDate = parseFormats(date, "2006-01-02", time.RFC3339)
			}

			if !credit && duration < 0 {
				duration = duration * -1
			}

			costs := &rosterv1.OffTimeCosts{
				OfftimeId:  offtimeId,
				RosterId:   rosterId,
				Costs:      durationpb.New(duration),
				IsVacation: isVacation,
				UserId:     userId,
				Date:       protoDate,
			}

			req := &rosterv1.AddOffTimeCostsRequest{
				AddCosts: []*rosterv1.OffTimeCosts{costs},
			}

			res, err := root.OffTime().AddOffTimeCosts(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to add costs: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&offtimeId, "offtime-entry", "", "The ID of the Offtime entry this costs belong to")
		f.StringVar(&rosterId, "roster", "", "The ID of the roster this costs belong to")
		f.StringVar(&userId, "user", "", "The ID of the user")
		f.DurationVar(&duration, "costs", 0, "The actual costs")
		f.BoolVar(&isVacation, "vacation", true, "Whether or not the costs count as vacation or time-off. Default: vacation")
		f.BoolVar(&credit, "credit", false, "Add positive costs (increasing vacation/time-off).")
		f.StringVar(&date, "date", "", "The effective date for the costs.")
	}

	return cmd
}

func GetOffTimeCostsCommand(root *cli.Root) *cobra.Command {
	var (
		userIds  []string
		allUsers bool
		paths    []string
	)

	cmd := &cobra.Command{
		Use: "get-costs",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.GetOffTimeCostsRequest{
				ReadMask: &fieldmaskpb.FieldMask{
					Paths: paths,
				},
			}

			if cmd.Flag("user").Changed || allUsers {
				req.ForUsers = &rosterv1.CostsForUsers{
					UserIds: userIds,
				}
			}

			res, err := root.OffTime().GetOffTimeCosts(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringSliceVar(&userIds, "user", nil, "A list of user IDs to query")
		f.BoolVar(&allUsers, "all-users", false, "Query all users")
		f.StringSliceVar(&paths, "fields", nil, "Which fields to return in the response")
	}

	return cmd
}

func DeleteOffTimeCostsCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "delete-costs",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.OffTime().DeleteOffTimeCosts(context.Background(), connect.NewRequest(&rosterv1.DeleteOffTimeCostsRequest{
				Ids: args,
			}))
			if err != nil {
				logrus.Fatalf("failed to delete off-time request: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}

func DeleteOffTimeRequestCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "delete",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.OffTime().DeleteOffTimeRequest(context.Background(), connect.NewRequest(&rosterv1.DeleteOffTimeRequestRequest{
				Id: args,
			}))
			if err != nil {
				logrus.Fatalf("failed to delete off-time request: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}

func CreateOffTimeRequestCommand(root *cli.Root) *cobra.Command {
	var (
		from        string
		to          string
		description string
		requestor   string
		reqType     string
	)
	cmd := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			var protoType rosterv1.OffTimeType
			switch reqType {
			case "auto":
				protoType = rosterv1.OffTimeType_OFF_TIME_TYPE_UNSPECIFIED
			case "time-off":
				protoType = rosterv1.OffTimeType_OFF_TIME_TYPE_TIME_OFF
			case "vacation":
				protoType = rosterv1.OffTimeType_OFF_TIME_TYPE_VACATION
			default:
				logrus.Fatalf("invalid value for --type: %q", reqType)
			}

			req := &rosterv1.CreateOffTimeRequestRequest{
				From:        parseFormats(from, "2006-01-02", time.RFC3339),
				To:          parseFormats(to, "2006-01-02", time.RFC3339),
				Description: description,
				RequestorId: requestor,
				RequestType: protoType,
			}

			res, err := root.OffTime().CreateOffTimeRequest(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to create off-time request: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&from, "from", "", "The date at which the off-time should start. Either YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS")
		f.StringVar(&to, "to", "", "The date at which the off-time should end. Either YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS")
		f.StringVar(&description, "description", "", "An optional description for management")
		f.StringVar(&requestor, "request-for", "", "The ID of the user for which the off-time request should be created.")
		f.StringVar(&reqType, "type", "auto", "The type of the request")
	}
	return cmd
}

func ApproveOrRejectCommand(root *cli.Root) *cobra.Command {
	var (
		approve bool
		comment string
	)
	cmd := &cobra.Command{
		Use:     "approve-reject",
		Aliases: []string{"approve", "reject"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if !cmd.Flag("approve").Changed {
				approve = cmd.CalledAs() == "approve"
			}

			req := &rosterv1.ApproveOrRejectRequest{
				Id:      args[0],
				Comment: comment,
			}

			if approve {
				req.Type = rosterv1.ApprovalRequestType_APPROVAL_REQUEST_TYPE_APPROVED
			} else {
				req.Type = rosterv1.ApprovalRequestType_APPROVAL_REQUEST_TYPE_REJECTED
			}

			res, err := root.OffTime().ApproveOrReject(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to approve/reject: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.BoolVar(&approve, "approve", true, "Approve or reject the request")
		f.StringVar(&comment, "comment", "", "An optional comment")
	}

	return cmd
}

func parseFormats(val string, formats ...string) *timestamppb.Timestamp {
	for _, f := range formats {
		t, err := time.ParseInLocation(f, val, time.Local)
		if err == nil {
			return timestamppb.New(t)
		}
	}

	logrus.Fatalf("%s does not match any supported time format", val)

	// can never be reached
	return nil
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
