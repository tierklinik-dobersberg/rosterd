package main

import (
	"context"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/sethvargo/go-envconfig"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/rosterd/client"
)

var (
	config struct {
		Server string `env:"ROSTERD_URL"`
		JWT    string `env:"ROSTERD_JWT"`
	}

	cli *client.Client
)

func getRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rosterctl [command]",
		Short: "Manage rosters, off-time requests and work-shifts",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if err := envconfig.Process(context.Background(), &config); err != nil {
				hclog.L().Error("failed to process environment variables", "error", err)
				os.Exit(1)
			}

			if config.Server == "" {
				hclog.L().Error("Either --server or ROSTERD_URL environment variable must be set")
				os.Exit(1)
			}

			if config.JWT == "" {
				hclog.L().Error("ROSTERD_JWT environment variable must be set")
				os.Exit(1)
			}

			cli = client.New(config.Server, config.JWT)
		},
	}

	flags := cmd.PersistentFlags()
	{
		flags.StringVarP(&config.Server, "server", "s", "", "The address of the rosterd server")
	}

	cmd.AddCommand(
		getWorkShiftCommand(),
		getRosterCommand(),
		getOffTimeCommand(),
	)

	return cmd
}

func main() {
	if err := getRootCommand().Execute(); err != nil {
		hclog.L().Error(err.Error())
		os.Exit(1)
	}
}
