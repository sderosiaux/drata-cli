package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

// Finding represents a single failing resource for a monitor.
type Finding struct {
	ID                 int     `json:"id"`
	ResourceExternalID string  `json:"resourceExternalId"`
	ResourceType       string  `json:"resourceType"`
	ResourceName       string  `json:"resourceName"`
	AccountID          string  `json:"accountId"`
	Region             string  `json:"region"`
	Status             string  `json:"status"`
	Description        string  `json:"description"`
	FailedAt           *string `json:"failedAt"`
	ConnectionName     string  `json:"connectionName"`
	IsExcluded         bool    `json:"isExcluded"`
}

type findingsResult struct {
	MonitorID int       `json:"monitorId"`
	Total     int       `json:"total"`
	Showing   int       `json:"showing"`
	Findings  []Finding `json:"findings"`
}

// CheckResult represents a single check execution in the monitor's history.
type CheckResult struct {
	ID        int     `json:"id"`
	Status    string  `json:"status"`
	CheckedAt *string `json:"checkedAt"`
	CreatedAt *string `json:"createdAt"`
	Passed    *int    `json:"passed"`
	Failed    *int    `json:"failed"`
	Error     *string `json:"error"`
}

// MonitorDetail represents the detailed response for a monitor,
// including its check result history.
type MonitorDetail struct {
	ID                int              `json:"id"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	CheckResultStatus string           `json:"checkResultStatus"`
	CheckStatus       string           `json:"checkStatus"`
	Priority          string           `json:"priority"`
	LastCheck         *string          `json:"lastCheck"`
	Controls          []MonitorControl `json:"controls"`
	CheckResults      []CheckResult    `json:"checkResults"`
}

type historyResult struct {
	MonitorID    int           `json:"monitorId"`
	MonitorName  string        `json:"monitorName"`
	Total        int           `json:"total"`
	Showing      int           `json:"showing"`
	CheckResults []CheckResult `json:"checkResults"`
}

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

	// findings — show failing resources for a monitor
	findingsCmd := &cobra.Command{
		Use:     "findings <id>",
		Short:   "Show findings (failing resources) for a monitor",
		Long:    "Lists the specific resources that are causing a monitor to fail, e.g. which S3 bucket, which ALB, which account.",
		Example: "  drata monitors findings 31\n  drata monitors findings 31 --json\n  drata monitors findings 31 --json --compact",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testID := args[0]
			c := client.New()
			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/public/workspaces/%d/monitors/%s/failures", wsID, testID)
			items, err := c.GetAll(path, nil)
			if err != nil {
				return fmt.Errorf("fetch findings for monitor %s: %w", testID, err)
			}

			var result findingsResult
			for _, raw := range items {
				var f Finding
				if err := json.Unmarshal(raw, &f); err != nil {
					continue
				}
				result.Findings = append(result.Findings, f)
			}
			result.MonitorID, _ = strconv.Atoi(testID)
			result.Total = len(result.Findings)
			result.Findings = output.LimitSlice(result.Findings)
			result.Showing = len(result.Findings)

			output.Print(result, formatFindings(result), compactFinding)
			return nil
		},
	}

	// history — show check history for a monitor
	historyCmd := &cobra.Command{
		Use:     "history <id>",
		Short:   "Show check history (pass/fail over time) for a monitor",
		Long:    "Shows when checks ran, whether they passed or failed, and pass/fail counts over time.",
		Example: "  drata monitors history 31\n  drata monitors history 31 --json\n  drata monitors history 31 --limit 10",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testID := args[0]
			c := client.New()
			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/public/workspaces/%d/monitors/%s/details", wsID, testID)
			raw, err := c.Get(path)
			if err != nil {
				return fmt.Errorf("fetch history for monitor %s: %w", testID, err)
			}

			var detail MonitorDetail
			if err := json.Unmarshal(raw, &detail); err != nil {
				return fmt.Errorf("parse monitor detail: %w", err)
			}

			var result historyResult
			result.MonitorID = detail.ID
			result.MonitorName = detail.Name
			result.CheckResults = detail.CheckResults
			result.Total = len(result.CheckResults)
			result.CheckResults = output.LimitSlice(result.CheckResults)
			result.Showing = len(result.CheckResults)

			output.Print(result, formatHistory(result), compactHistory)
			return nil
		},
	}

	cmd.AddCommand(listCmd, failingCmd, getCmd, forControlCmd, findingsCmd, historyCmd)
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

func compactFinding(v any) any {
	switch f := v.(type) {
	case Finding:
		return map[string]any{
			"resourceExternalId": f.ResourceExternalID,
			"resourceType":       f.ResourceType,
			"status":             f.Status,
			"accountId":          f.AccountID,
			"region":             f.Region,
		}
	case findingsResult:
		compact := make([]any, len(f.Findings))
		for i, finding := range f.Findings {
			compact[i] = compactFinding(finding)
		}
		return map[string]any{"monitorId": f.MonitorID, "total": f.Total, "showing": f.Showing, "findings": compact}
	}
	return v
}

func compactHistory(v any) any {
	switch h := v.(type) {
	case CheckResult:
		ts := ""
		if h.CheckedAt != nil {
			ts = *h.CheckedAt
		} else if h.CreatedAt != nil {
			ts = *h.CreatedAt
		}
		return map[string]any{"status": h.Status, "checkedAt": ts}
	case historyResult:
		compact := make([]any, len(h.CheckResults))
		for i, cr := range h.CheckResults {
			compact[i] = compactHistory(cr)
		}
		return map[string]any{
			"monitorId":    h.MonitorID,
			"monitorName":  h.MonitorName,
			"total":        h.Total,
			"showing":      h.Showing,
			"checkResults": compact,
		}
	}
	return v
}

func formatFindings(r findingsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  monitor=%d  total=%d  showing=%d\n\n",
		output.Bold("Findings"), r.MonitorID, r.Total, r.Showing)

	if len(r.Findings) == 0 {
		fmt.Fprintf(&sb, "  %s\n", output.Green("No findings — all resources passing."))
		return sb.String()
	}

	for _, f := range r.Findings {
		status := f.Status
		if status == "" {
			status = "FAILED"
		}
		excludedTag := ""
		if f.IsExcluded {
			excludedTag = output.Dim(" [excluded]")
		}
		name := f.ResourceName
		if name == "" {
			name = f.ResourceExternalID
		}
		fmt.Fprintf(&sb, "  %s  %s%s\n",
			output.Col(output.StatusColor(status), 22),
			name,
			excludedTag)

		var details []string
		if f.ResourceType != "" {
			details = append(details, "type="+f.ResourceType)
		}
		if f.AccountID != "" {
			details = append(details, "account="+f.AccountID)
		}
		if f.Region != "" {
			details = append(details, "region="+f.Region)
		}
		if f.ConnectionName != "" {
			details = append(details, "connection="+f.ConnectionName)
		}
		if f.FailedAt != nil {
			details = append(details, "failed="+shortTime(*f.FailedAt))
		}
		if len(details) > 0 {
			fmt.Fprintf(&sb, "       %s\n", output.Dim(strings.Join(details, "  ")))
		}
		if f.Description != "" {
			fmt.Fprintf(&sb, "       %s\n", output.Dim(f.Description))
		}
	}
	return sb.String()
}

func formatHistory(r historyResult) string {
	var sb strings.Builder
	header := r.MonitorName
	if header == "" {
		header = fmt.Sprintf("Monitor %d", r.MonitorID)
	}
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n\n",
		output.Bold("Check History: "+header), r.Total, r.Showing)

	if len(r.CheckResults) == 0 {
		fmt.Fprintf(&sb, "  %s\n", output.Dim("No check history available."))
		return sb.String()
	}

	for _, cr := range r.CheckResults {
		ts := ""
		if cr.CheckedAt != nil {
			ts = shortTime(*cr.CheckedAt)
		} else if cr.CreatedAt != nil {
			ts = shortTime(*cr.CreatedAt)
		}
		counts := ""
		if cr.Passed != nil || cr.Failed != nil {
			p, f := 0, 0
			if cr.Passed != nil {
				p = *cr.Passed
			}
			if cr.Failed != nil {
				f = *cr.Failed
			}
			counts = fmt.Sprintf("  passed=%s failed=%s",
				output.Green(fmt.Sprint(p)),
				output.Red(fmt.Sprint(f)))
		}
		errMsg := ""
		if cr.Error != nil && *cr.Error != "" {
			errMsg = "  " + output.Red("error: "+*cr.Error)
		}
		fmt.Fprintf(&sb, "  %s  %s%s%s\n",
			output.Col(output.Dim(ts), 22),
			output.StatusColor(cr.Status),
			counts,
			errMsg)
	}
	return sb.String()
}

// shortTime formats an ISO timestamp for display, showing date and time.
func shortTime(ts string) string {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	return ts
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
