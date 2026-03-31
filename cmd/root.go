package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/config"
	"github.com/sderosiaux/drata-cli/internal/output"
)

const version = "0.1.0"

var regionFlag string

var rootCmd = &cobra.Command{
	Use:   "drata",
	Short: "Drata compliance platform CLI",
	Long: `CLI for the Drata compliance platform.

Authentication:
  drata auth set-key <api-key>     # persist to $XDG_CONFIG_HOME/drata-cli/config.yaml
  drata auth check                 # verify key + show workspace
  export DRATA_API_KEY=<api-key>   # env var overrides stored key

Optional:
  export DRATA_REGION=us|eu|apac   (default: us)

LLM/script usage:
  drata summary --json --compact
  drata controls failing --json
  drata monitors failing --json
  drata monitors get <id> --json`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		compactFlag, _ := cmd.Flags().GetBool("compact")
		limitFlag, _ := cmd.Flags().GetInt("limit")
		noColorFlag, _ := cmd.Flags().GetBool("no-color")

		if noColorFlag {
			color.NoColor = true
		}
		output.SetJSON(jsonFlag)
		output.SetCompact(compactFlag)
		output.SetLimit(limitFlag)

		if cmd.Annotations["skip_auth"] == "true" {
			return nil
		}
		return config.Init(regionFlag)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Fail(err)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON (for LLM/scripts)")
	rootCmd.PersistentFlags().Bool("compact", false, "Minimal fields only (requires --json)")
	rootCmd.PersistentFlags().Int("limit", 0, "Limit number of results")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output (also: NO_COLOR env)")
	rootCmd.PersistentFlags().StringVar(&regionFlag, "region", "", "API region: us, eu, apac (default: us or DRATA_REGION)")

	rootCmd.AddCommand(
		authCmd(),
		controlsCmd(),
		monitorsCmd(),
		personnelCmd(),
		policiesCmd(),
		connectionsCmd(),
		vendorsCmd(),
		devicesCmd(),
		evidenceCmd(),
		eventsCmd(),
		assetsCmd(),
		usersCmd(),
		summaryCmd(),
	)
}
