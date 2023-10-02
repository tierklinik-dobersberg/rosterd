package cmds

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func ConstraintCommand(root *cli.Root) *cobra.Command {
	var (
		userIds []string
		roleIds []string
	)

	cmd := &cobra.Command{
		Use: "constraint",
		Run: func(cmd *cobra.Command, args []string) {
			req := &rosterv1.FindConstraintsRequest{
				UserIds: userIds,
				RoleIds: roleIds,
			}

			res, err := root.Constraints().FindConstraints(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	cmd.Flags().StringSliceVar(&userIds, "user", nil, "")
	cmd.Flags().StringSliceVar(&roleIds, "role", nil, "")

	cmd.AddCommand(
		CreateConstraintCommand(root),
		UpdateConstraintCommand(root),
		DeleteConstraintCommand(root),
	)

	return cmd
}

func CreateConstraintCommand(root *cli.Root) *cobra.Command {
	req := &rosterv1.CreateConstraintRequest{}
	cmd := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.Constraints().CreateConstraint(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&req.Description, "description", "", "")
		f.StringSliceVar(&req.RoleIds, "role", nil, "")
		f.StringSliceVar(&req.UserIds, "user", nil, "")
		f.StringVar(&req.Expression, "expr", "", "")
		f.BoolVar(&req.Deny, "deny", false, "")
		f.BoolVar(&req.Hard, "hard", false, "")
		f.BoolVar(&req.RosterOnly, "roster-only", false, "")
		f.Float32Var(&req.Penalty, "penalty", 0, "")
	}

	return cmd
}

func UpdateConstraintCommand(root *cli.Root) *cobra.Command {
	req := &rosterv1.UpdateConstraintRequest{
		WriteMask: &fieldmaskpb.FieldMask{},
	}
	cmd := &cobra.Command{
		Use:  "create",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			req.Id = args[0]

			flags := [][]string{
				{"description", "description"},
				{"role", "role_ids"},
				{"user", "user_ids"},
				{"expr", "expression"},
				{"hard", "hard"},
				{"deny", "deny"},
				{"roster_only", "roster_only"},
			}

			for _, f := range flags {
				if cmd.Flag(f[0]).Changed {
					req.WriteMask.Paths = append(req.WriteMask.Paths, f[1])
				}
			}

			res, err := root.Constraints().UpdateConstraint(context.Background(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&req.Description, "description", "", "")
		f.StringSliceVar(&req.RoleIds, "role", nil, "")
		f.StringSliceVar(&req.UserIds, "user", nil, "")
		f.StringVar(&req.Expression, "expr", "", "")
		f.BoolVar(&req.Deny, "deny", false, "")
		f.BoolVar(&req.Hard, "hard", false, "")
		f.BoolVar(&req.RosterOnly, "roster-only", false, "")
		f.Float32Var(&req.Penalty, "penalty", 0, "")
	}

	return cmd

}

func DeleteConstraintCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "rm", "remove"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.Constraints().DeleteConstraint(context.Background(), connect.NewRequest(&rosterv1.DeleteConstraintRequest{
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
