package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type MonitorControl struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
}

type Monitor struct {
	ID                int              `json:"id"`
	Name              string           `json:"name"`
	CheckResultStatus string           `json:"checkResultStatus"`
	Priority          string           `json:"priority"`
	LastCheck         *string          `json:"lastCheck"`
	Controls          []MonitorControl `json:"controls"`
}

type monitorInstanceDetail struct {
	FailedTestDescription         string `json:"failedTestDescription"`
	RemedyDescription             string `json:"remedyDescription"`
	EvidenceCollectionDescription string `json:"evidenceCollectionDescription"`
}

type MonitorInstance struct {
	ID                int                     `json:"id"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	CheckResultStatus string                  `json:"checkResultStatus"`
	CheckStatus       string                  `json:"checkStatus"`
	Priority          string                  `json:"priority"`
	LastCheck         *string                 `json:"lastCheck"`
	Controls          []MonitorControl        `json:"controls"`
	MonitorInstances  []monitorInstanceDetail `json:"monitorInstances"`
}

type monitorsResult struct {
	Total    int       `json:"total"`
	Showing  int       `json:"showing"`
	Monitors []Monitor `json:"monitors"`
}

func monitorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitors",
		Short: "Manage compliance monitors",
	}

	var statusFlag string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all monitors",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{}
			if statusFlag != "" {
				params.Set("checkResultStatus", statusFlag)
			}
			items, err := c.GetAll("/public/monitors", params)
			if err != nil {
				return err
			}

			var result monitorsResult
			for _, raw := range items {
				var m Monitor
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				result.Monitors = append(result.Monitors, m)
			}
			result.Total = len(items)
			result.Monitors = output.LimitSlice(result.Monitors)
			result.Showing = len(result.Monitors)

			output.Print(result, formatMonitors(result), compactMonitor)
			return nil
		},
	}
	listCmd.Flags().StringVar(&statusFlag, "status", "", "Filter: PASSED, FAILED, NOT_TESTED")

	failingCmd := &cobra.Command{
		Use:     "failing",
		Short:   "List FAILED monitors with affected controls",
		Example: "  drata monitors failing\n  drata monitors failing --json --compact",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{"checkResultStatus": []string{"FAILED"}}
			items, err := c.GetAll("/public/monitors", params)
			if err != nil {
				return err
			}

			var result monitorsResult
			for _, raw := range items {
				var m Monitor
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				result.Monitors = append(result.Monitors, m)
			}
			result.Total = len(items)
			result.Monitors = output.LimitSlice(result.Monitors)
			result.Showing = len(result.Monitors)

			output.Print(result, formatMonitorsFailing(result), compactMonitor)
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get monitor details by ID (includes failure description and remedy)",
		Example: "  drata monitors get 31\n  drata monitors get 31 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// GET /public/monitors/{id} doesn't exist in Drata public API.
			// Fetch full list (which includes monitorInstances) and find by ID.
			targetID := args[0]
			c := client.New()
			items, err := c.GetAll("/public/monitors", nil)
			if err != nil {
				return err
			}
			for _, raw := range items {
				var m MonitorInstance
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				if fmt.Sprint(m.ID) == targetID {
					output.Print(m, formatMonitorInstance(m), func(v any) any {
						if mi, ok := v.(MonitorInstance); ok {
							return map[string]any{
								"id":     mi.ID,
								"name":   mi.Name,
								"status": mi.CheckResultStatus,
							}
						}
						return v
					})
					return nil
				}
			}
			return fmt.Errorf("monitor %q not found", targetID)
		},
	}

	// for-control — list monitors that reference a given control code
	forControlCmd := &cobra.Command{
		Use:   "for-control <code>",
		Short: "List monitors linked to a control code (e.g. DCF-71)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code := args[0]
			c := client.New()
			items, err := c.GetAll("/public/monitors", nil)
			if err != nil {
				return err
			}

			var result monitorsResult
			for _, raw := range items {
				var m Monitor
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				for _, ctrl := range m.Controls {
					if ctrl.Code == code {
						result.Monitors = append(result.Monitors, m)
						break
					}
				}
			}
			result.Total = len(items)
			result.Monitors = output.LimitSlice(result.Monitors)
			result.Showing = len(result.Monitors)

			output.Print(result, formatMonitors(result), compactMonitor)
			return nil
		},
	}

	cmd.AddCommand(listCmd, failingCmd, getCmd, forControlCmd)
	return cmd
}

func compactMonitor(v any) any {
	switch m := v.(type) {
	case Monitor:
		return map[string]any{"id": m.ID, "name": m.Name, "status": m.CheckResultStatus}
	case monitorsResult:
		compact := make([]any, len(m.Monitors))
		for i, mon := range m.Monitors {
			compact[i] = compactMonitor(mon)
		}
		return map[string]any{"total": m.Total, "showing": m.Showing, "monitors": compact}
	}
	return v
}

func formatMonitors(r monitorsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n\n",
		output.Bold("Monitors"), r.Total, r.Showing)
	for _, m := range r.Monitors {
		lastCheck := "never"
		if m.LastCheck != nil {
			lastCheck = *m.LastCheck
		}
		fmt.Fprintf(&sb, "  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(m.ID), 8),
			output.Col(output.StatusColor(m.CheckResultStatus), 22),
			output.Col(output.Dim(lastCheck), 26),
			m.Name)
	}
	return sb.String()
}

func formatMonitorsFailing(r monitorsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  count=%d\n\n", output.Bold(output.Red("Failing Monitors")), r.Showing)
	for _, m := range r.Monitors {
		codes := make([]string, len(m.Controls))
		for i, c := range m.Controls {
			codes[i] = c.Code
		}
		fmt.Fprintf(&sb, "  %s  %s\n",
			output.Col(fmt.Sprint(m.ID), 8),
			m.Name)
		if len(codes) > 0 {
			fmt.Fprintf(&sb, "       controls: %s\n", output.Cyan(strings.Join(codes, ", ")))
		}
	}
	return sb.String()
}

func formatMonitorInstance(m MonitorInstance) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  [%d]\n", output.Bold(m.Name), m.ID)
	fmt.Fprintf(&sb, "Status:   %s\n", output.StatusColor(m.CheckResultStatus))
	fmt.Fprintf(&sb, "Priority: %s\n", m.Priority)
	if m.LastCheck != nil {
		fmt.Fprintf(&sb, "Last check: %s\n", *m.LastCheck)
	}
	if len(m.MonitorInstances) > 0 {
		inst := m.MonitorInstances[0]
		if inst.FailedTestDescription != "" {
			fmt.Fprintf(&sb, "\n%s\n%s\n", output.Bold("Failed Test:"), inst.FailedTestDescription)
		}
		if inst.RemedyDescription != "" {
			fmt.Fprintf(&sb, "\n%s\n%s\n", output.Bold("Remedy:"), inst.RemedyDescription)
		}
		if inst.EvidenceCollectionDescription != "" {
			fmt.Fprintf(&sb, "\n%s\n%s\n", output.Bold("Evidence collection:"), inst.EvidenceCollectionDescription)
		}
	}
	if len(m.Controls) > 0 {
		codes := make([]string, len(m.Controls))
		for i, c := range m.Controls {
			codes[i] = c.Code
		}
		fmt.Fprintf(&sb, "\nControls: %s\n", output.Cyan(strings.Join(codes, ", ")))
	}
	return sb.String()
}
