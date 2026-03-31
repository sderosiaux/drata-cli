package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/config"
	"github.com/sderosiaux/drata-cli/internal/output"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	setKeyCmd := &cobra.Command{
		Use:   "set-key <api-key>",
		Short: "Persist API key to ~/.config/drata-cli/config.yaml",
		Long: `Writes the API key to ~/.config/drata-cli/config.yaml so you don't
need to set DRATA_API_KEY on every invocation.

The env var DRATA_API_KEY always takes precedence over the stored key.`,
		Example: "  drata auth set-key 5d595aeb-dcb0-4221-8767-5de540fb0d10",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"skip_auth": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if key == "" {
				return fmt.Errorf("api-key cannot be empty")
			}
			path, err := config.ConfigPath()
			if err != nil {
				return err
			}
			if err := config.WriteKey(key); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(os.Stdout, "API key saved to %s\n", path)
			_, _ = fmt.Fprintln(os.Stdout, "Run 'drata auth check' to verify.")
			return nil
		},
	}

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Verify the current API key and show workspace info",
		Example: `  drata auth check
  drata auth check --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			keySource := "env"
			if os.Getenv("DRATA_API_KEY") == "" {
				keySource = "config file"
			}

			c := client.New()
			raw, err := c.Get("/public/workspaces")
			if err != nil {
				return fmt.Errorf("auth check failed: %w", err)
			}

			var pr struct {
				Data []struct {
					Name    string `json:"name"`
					Primary bool   `json:"primary"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &pr); err != nil {
				return fmt.Errorf("parse workspaces: %w", err)
			}

			type result struct {
				Status    string `json:"status"`
				Workspace string `json:"workspace"`
				Region    string `json:"region"`
				KeySource string `json:"key_source"`
			}

			workspace := ""
			for _, w := range pr.Data {
				if w.Primary {
					workspace = w.Name
					break
				}
			}
			if workspace == "" && len(pr.Data) > 0 {
				workspace = pr.Data[0].Name
			}

			r := result{
				Status:    "OK",
				Workspace: workspace,
				Region:    config.Region,
				KeySource: keySource,
			}

			formatted := fmt.Sprintf("%s\nWorkspace: %s\nRegion:    %s\nKey from:  %s\n",
				output.Green("✓ Authenticated"),
				output.Bold(workspace),
				output.Cyan(config.Region),
				output.Dim(keySource),
			)
			output.Print(r, formatted, nil)
			return nil
		},
	}

	cmd.AddCommand(setKeyCmd, checkCmd)
	return cmd
}
