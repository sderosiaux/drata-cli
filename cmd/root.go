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
	Use:     "drata",
	Short:   "Drata compliance platform CLI",
	Long:    "CLI for the Drata compliance platform. Covers controls, monitors, personnel, policies, vendors, devices, evidence, events, assets, users and more.",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		compactFlag, _ := cmd.Flags().GetBool("compact")
		limitFlag, _ := cmd.Flags().GetInt("limit")
		noColorFlag, _ := cmd.Flags().GetBool("no-color")

		if noColorFlag {
			color.NoColor = true
		}
		output.SetJSON(jsonFlag) // also sets color.NoColor = true
		output.SetCompact(compactFlag)
		output.SetLimit(limitFlag)

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
