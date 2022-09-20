package main

import (
	"encoding/json"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func getConstraintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "constraint",
		Short: "Manage roster constraints",
	}

	cmd.AddCommand(
		getCreateConstraintCommand(),
		getDeleteConstraintCommand(),
		getFindConstraintCommand(),
	)

	return cmd
}

func getCreateConstraintCommand() *cobra.Command {
	var staff []string
	var roles []string

	var constraint structs.Constraint

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new roster constraint",
		Run: func(cmd *cobra.Command, args []string) {
			for _, s := range staff {
				constraint.AppliesTo = append(constraint.AppliesTo, "staff:"+s)
			}
			for _, r := range roles {
				constraint.AppliesTo = append(constraint.AppliesTo, "role:"+r)
			}

			if err := cli.CreateConstraint(cmd.Context(), constraint); err != nil {
				hclog.L().Error("failed to create constraint", "error", err)
				os.Exit(1)
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.StringVar(&constraint.Description, "name", "", "The name/description of the constraint")
		flags.StringVar(&constraint.Expression, "expr", "", "The expression to evaluate the constraint")
		flags.StringSliceVar(&roles, "role", nil, "A list of roles this constraints applies to")
		flags.StringSliceVar(&staff, "staff", nil, "A list of staff identifiers this constraint applies to")
		flags.BoolVar(&constraint.Hard, "hard", false, "Whether or not this is a hard constraint")
		flags.BoolVar(&constraint.Deny, "deny", false, "Whether or not this constaint evaluates to deny or allow")
		flags.BoolVar(&constraint.RosterOnly, "roster", false, "Only evaluate constraint against the complete roster")
		flags.IntVar(&constraint.Penalty, "penalty", 0, "A penalty when the constraint is violated")
	}

	return cmd
}

func getDeleteConstraintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a roster constraint by id",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			hasError := false
			for _, id := range args {
				if err := cli.DeleteConstraint(cmd.Context(), id); err != nil {
					hclog.L().Error("failed to delete constraint", "id", id, "error", err)
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

func getFindConstraintCommand() *cobra.Command {
	var (
		staffFilter []string
		roleFilter  []string
		jsonOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Find and list roster constraints",
		Run: func(cmd *cobra.Command, args []string) {
			res, err := cli.FindConstraints(cmd.Context(), staffFilter, roleFilter)
			if err != nil {
				hclog.L().Error("failed to search for constraints", "error", err)
				os.Exit(1)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "    ")
				enc.Encode(res)

				return
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.StringSliceVar(&staffFilter, "staff", nil, "Only display constraints that apply to the given staffs")
		flags.StringSliceVar(&roleFilter, "role", nil, "Only display constraints that apply to the given roles")
		flags.BoolVar(&jsonOutput, "json", false, "Display result in JSON")
	}

	return cmd
}
