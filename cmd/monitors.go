package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

// Finding represents a single failing resource for a monitor.
// Matches the Drata v1 API response from /public/workspaces/{wsID}/monitors/{testId}/failures.
type Finding struct {
	ID             string          `json:"id"`
	ProviderName   string          `json:"providerName"`
	ClientID       string          `json:"clientId"`
	ClientAlias    string          `json:"clientAlias"`
	ResourceName   string          `json:"resourceName"`
	DisplayName    string          `json:"displayName"`
	AccountID      string          `json:"accountId"`
	AccountName    *string         `json:"accountName"`
	FailingMessage string          `json:"failingMessage"`
	ResourceArn    string          `json:"resourceArn"`
	Cause          json.RawMessage `json:"cause"`
}

type findingsResult struct {
	MonitorID   int              `json:"monitorId"`
	MonitorName string           `json:"monitorName"`
	Controls    []MonitorControl `json:"controls,omitempty"`
	Frameworks  []string         `json:"frameworks,omitempty"`
	Total       int              `json:"total"`
	Showing     int              `json:"showing"`
	Findings    []Finding        `json:"findings"`
}

// MetadataResource represents a pass/fail resource in monitor instance metadata.
type MetadataResource struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Email       *string `json:"email"`
}

// CheckMetadata represents one connection's check result in a monitor instance.
type CheckMetadata struct {
	CheckResultStatus string             `json:"checkResultStatus"`
	Type              string             `json:"type"`
	Source            string             `json:"source"`
	ConnectionID      int                `json:"connectionId"`
	ClientID          string             `json:"clientId"`
	ClientAlias       string             `json:"clientAlias"`
	ClientType        string             `json:"clientType"`
	Pass              []MetadataResource `json:"pass"`
	Fail              []MetadataResource `json:"fail"`
	Exclusions        []MetadataResource `json:"exclusions"`
	Message           *string            `json:"message"`
}

// MonitorInstanceDetail represents a monitor instance in the details response.
type MonitorInstanceDetail struct {
	ID                            int             `json:"id"`
	CheckResultStatus             string          `json:"checkResultStatus"`
	CheckFrequency                string          `json:"checkFrequency"`
	FailedTestDescription         string          `json:"failedTestDescription"`
	RemedyDescription             string          `json:"remedyDescription"`
	EvidenceCollectionDescription string          `json:"evidenceCollectionDescription"`
	Metadata                      []CheckMetadata `json:"metadata"`
}

// MonitorDetail represents the detailed response for a monitor from the details endpoint.
type MonitorDetail struct {
	ID                int                     `json:"id"`
	TestID            int                     `json:"testId"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	CheckResultStatus string                  `json:"checkResultStatus"`
	CheckStatus       string                  `json:"checkStatus"`
	Priority          string                  `json:"priority"`
	LastCheck         *string                 `json:"lastCheck"`
	Controls          []MonitorControl        `json:"controls"`
	MonitorInstances  []MonitorInstanceDetail `json:"monitorInstances"`
}

type historyResult struct {
	MonitorID   int              `json:"monitorId"`
	MonitorName string           `json:"monitorName"`
	Controls    []MonitorControl `json:"controls,omitempty"`
	Frameworks  []string         `json:"frameworks,omitempty"`
	Connections []connectionResult `json:"connections"`
}

type connectionResult struct {
	Source      string `json:"source"`
	ClientID    string `json:"clientId"`
	ClientAlias string `json:"clientAlias"`
	Status      string `json:"status"`
	Passed      int    `json:"passed"`
	Failed      int    `json:"failed"`
	Excluded    int    `json:"excluded"`
}

type retestResult struct {
	MonitorID   int    `json:"monitorId"`
	MonitorName string `json:"monitorName"`
	TestID      int    `json:"testId"`
	Status      string `json:"status"`
}

type MonitorControl struct {
	ID         int      `json:"id"`
	Code       string   `json:"code"`
	Frameworks []string `json:"frameworkTags,omitempty"`
}

// controlFrameworkMap fetches all controls from the API and returns a map
// from control ID to framework tags (e.g., ["SOC_2", "PCI_DSS", "HIPAA"]).
// Returns an empty map on error (best-effort — never blocks display).
func controlFrameworkMap(c *client.Client) map[int][]string {
	items, err := c.GetAll("/public/controls", nil)
	if err != nil {
		return map[int][]string{}
	}
	m := make(map[int][]string, len(items))
	for _, raw := range items {
		var ctrl struct {
			ID         int      `json:"id"`
			Frameworks []string `json:"frameworkTags"`
		}
		if err := json.Unmarshal(raw, &ctrl); err != nil {
			continue
		}
		if len(ctrl.Frameworks) > 0 {
			m[ctrl.ID] = ctrl.Frameworks
		}
	}
	return m
}

// enrichControlsWithFrameworks annotates MonitorControl slices with framework
// tags from the pre-fetched map. Controls that already have inline frameworks
// (from the API) are left untouched.
func enrichControlsWithFrameworks(controls []MonitorControl, fwMap map[int][]string) {
	for i := range controls {
		if len(controls[i].Frameworks) == 0 {
			if fws, ok := fwMap[controls[i].ID]; ok {
				controls[i].Frameworks = fws
			}
		}
	}
}

// frameworkDisplay formats framework tags for human display.
// "SOC_2" → "SOC 2", "PCI_DSS" → "PCI DSS", etc.
func frameworkDisplay(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = strings.ReplaceAll(t, "_", " ")
	}
	return strings.Join(names, ", ")
}

// controlCodesWithFrameworks formats control codes with framework names.
// e.g., "DCF-71 (SOC 2, PCI DSS), DCF-85 (HIPAA)"
func controlCodesWithFrameworks(controls []MonitorControl) string {
	parts := make([]string, len(controls))
	for i, c := range controls {
		if len(c.Frameworks) > 0 {
			parts[i] = fmt.Sprintf("%s (%s)", c.Code, frameworkDisplay(c.Frameworks))
		} else {
			parts[i] = c.Code
		}
	}
	return strings.Join(parts, ", ")
}

// uniqueFrameworks returns the deduplicated set of framework tags across all controls.
func uniqueFrameworks(controls []MonitorControl) []string {
	seen := map[string]bool{}
	var result []string
	for _, c := range controls {
		for _, f := range c.Frameworks {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

type Monitor struct {
	ID                int              `json:"id"`
	TestID            int              `json:"testId"`
	Name              string           `json:"name"`
	CheckResultStatus string           `json:"checkResultStatus"`
	Priority          string           `json:"priority"`
	LastCheck         *string          `json:"lastCheck"`
	Controls          []MonitorControl `json:"controls"`
}

// lookupMonitor fetches all monitors and returns the one matching the given
// instance ID. This is needed because the failures and details endpoints
// require testId (the template ID), not the instance id.
func lookupMonitor(c *client.Client, instanceID string) (*Monitor, error) {
	items, err := c.GetAll("/public/monitors", nil)
	if err != nil {
		return nil, err
	}
	for _, raw := range items {
		var m Monitor
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if fmt.Sprint(m.ID) == instanceID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("monitor %q not found", instanceID)
}

type monitorInstanceSummary struct {
	FailedTestDescription         string `json:"failedTestDescription"`
	RemedyDescription             string `json:"remedyDescription"`
	EvidenceCollectionDescription string `json:"evidenceCollectionDescription"`
}

type MonitorInstance struct {
	ID                int                     `json:"id"`
	TestID            int                     `json:"testId"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	CheckResultStatus string                  `json:"checkResultStatus"`
	CheckStatus       string                  `json:"checkStatus"`
	Priority          string                  `json:"priority"`
	LastCheck         *string                 `json:"lastCheck"`
	Controls          []MonitorControl        `json:"controls"`
	MonitorInstances  []monitorInstanceSummary `json:"monitorInstances"`
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
		Short: "List all monitors with compliance framework context",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()

			fwMap := controlFrameworkMap(c)

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
				enrichControlsWithFrameworks(m.Controls, fwMap)
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
		Short:   "List FAILED monitors with affected controls and compliance frameworks",
		Example: "  drata monitors failing\n  drata monitors failing --json --compact",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()

			// Fetch framework mapping for enrichment
			fwMap := controlFrameworkMap(c)

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
				enrichControlsWithFrameworks(m.Controls, fwMap)
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
		Short:   "Get monitor details by ID (includes failure description, remedy, and frameworks)",
		Example: "  drata monitors get 31\n  drata monitors get 31 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// GET /public/monitors/{id} doesn't exist in Drata public API.
			// Fetch full list (which includes monitorInstances) and find by ID.
			targetID := args[0]
			c := client.New()

			// Fetch controls → framework mapping in parallel with monitors
			fwMap := controlFrameworkMap(c)

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
					enrichControlsWithFrameworks(m.Controls, fwMap)
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
		Short: "List monitors linked to a control code (e.g. DCF-71), with frameworks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code := args[0]
			c := client.New()

			fwMap := controlFrameworkMap(c)

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
						enrichControlsWithFrameworks(m.Controls, fwMap)
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
		Short:   "Show findings (failing resources) for a monitor, with compliance framework context",
		Long:    "Lists the specific resources that are causing a monitor to fail, e.g. which S3 bucket, which ALB, which account. Shows which compliance frameworks are affected.",
		Example: "  drata monitors findings 128\n  drata monitors findings 128 --json\n  drata monitors findings 128 --json --compact",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			monitorID := args[0]
			c := client.New()

			// Fetch framework mapping for enrichment
			fwMap := controlFrameworkMap(c)

			// Look up monitor to get testId (the failures endpoint uses testId, not instance id)
			mon, err := lookupMonitor(c, monitorID)
			if err != nil {
				return err
			}
			enrichControlsWithFrameworks(mon.Controls, fwMap)

			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			// The failures endpoint requires a "type" param and uses testId.
			// It does not support limit/page pagination.
			path := fmt.Sprintf("/public/workspaces/%d/monitors/%d/failures?type=FAILED", wsID, mon.TestID)
			raw, err := c.Get(path)
			if err != nil {
				return fmt.Errorf("fetch findings for monitor %s: %w", monitorID, err)
			}

			var envelope struct {
				Data []Finding `json:"data"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return fmt.Errorf("parse findings: %w", err)
			}

			var result findingsResult
			result.MonitorID = mon.ID
			result.MonitorName = mon.Name
			result.Controls = mon.Controls
			result.Frameworks = uniqueFrameworks(mon.Controls)
			result.Total = len(envelope.Data)
			result.Findings = output.LimitSlice(envelope.Data)
			result.Showing = len(result.Findings)

			output.Print(result, formatFindings(result), compactFinding)
			return nil
		},
	}

	// history — show check details for a monitor (per-connection pass/fail breakdown)
	historyCmd := &cobra.Command{
		Use:     "history <id>",
		Short:   "Show check details per connection for a monitor",
		Long:    "Shows the per-connection check results: which AWS accounts/connections are passing or failing, with resource counts. Includes compliance frameworks.",
		Example: "  drata monitors history 128\n  drata monitors history 128 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			monitorID := args[0]
			c := client.New()

			// Fetch framework mapping for enrichment
			fwMap := controlFrameworkMap(c)

			// Look up monitor to get testId (the details endpoint uses testId)
			mon, err := lookupMonitor(c, monitorID)
			if err != nil {
				return err
			}
			enrichControlsWithFrameworks(mon.Controls, fwMap)

			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/public/workspaces/%d/monitors/%d/details", wsID, mon.TestID)
			raw, err := c.Get(path)
			if err != nil {
				return fmt.Errorf("fetch details for monitor %s: %w", monitorID, err)
			}

			var detail MonitorDetail
			if err := json.Unmarshal(raw, &detail); err != nil {
				return fmt.Errorf("parse monitor detail: %w", err)
			}

			var result historyResult
			result.MonitorID = detail.ID
			result.MonitorName = detail.Name
			result.Controls = mon.Controls
			result.Frameworks = uniqueFrameworks(mon.Controls)

			// Extract per-connection check results from monitor instances metadata
			for _, inst := range detail.MonitorInstances {
				for _, meta := range inst.Metadata {
					result.Connections = append(result.Connections, connectionResult{
						Source:      meta.Source,
						ClientID:    meta.ClientID,
						ClientAlias: meta.ClientAlias,
						Status:      meta.CheckResultStatus,
						Passed:      len(meta.Pass),
						Failed:      len(meta.Fail),
						Excluded:    len(meta.Exclusions),
					})
				}
			}

			output.Print(result, formatHistory(result), compactHistory)
			return nil
		},
	}

	// retest — trigger a retest for a monitor
	retestCmd := &cobra.Command{
		Use:     "retest <id>",
		Short:   "Trigger a retest for a monitor",
		Long:    "Triggers an immediate retest of a monitor's autopilot check. The monitor ID is the instance ID shown in 'drata monitors list'.",
		Example: "  drata monitors retest 128\n  drata monitors retest 128 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			monitorID := args[0]
			c := client.New()

			// Look up monitor to get testId and name
			mon, err := lookupMonitor(c, monitorID)
			if err != nil {
				return err
			}

			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/public/workspaces/%d/autopilot/%d/retest", wsID, mon.TestID)
			_, err = c.Post(path)
			if err != nil {
				return fmt.Errorf("retest monitor %s (%s): %w", monitorID, mon.Name, err)
			}

			result := retestResult{
				MonitorID:   mon.ID,
				MonitorName: mon.Name,
				TestID:      mon.TestID,
				Status:      "retest triggered",
			}

			output.Print(result, formatRetest(result), func(v any) any {
				if r, ok := v.(retestResult); ok {
					return map[string]any{
						"monitorId":   r.MonitorID,
						"monitorName": r.MonitorName,
						"status":      r.Status,
					}
				}
				return v
			})
			return nil
		},
	}

	cmd.AddCommand(listCmd, failingCmd, getCmd, forControlCmd, findingsCmd, historyCmd, retestCmd)
	return cmd
}

func compactMonitor(v any) any {
	switch m := v.(type) {
	case Monitor:
		fws := uniqueFrameworks(m.Controls)
		result := map[string]any{"id": m.ID, "name": m.Name, "status": m.CheckResultStatus}
		if len(fws) > 0 {
			result["frameworks"] = fws
		}
		return result
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
		fws := uniqueFrameworks(m.Controls)
		fwStr := ""
		if len(fws) > 0 {
			fwStr = "  " + output.Dim("["+frameworkDisplay(fws)+"]")
		}
		fmt.Fprintf(&sb, "  %s  %s  %s  %s%s\n",
			output.Col(fmt.Sprint(m.ID), 8),
			output.Col(output.StatusColor(m.CheckResultStatus), 22),
			output.Col(output.Dim(lastCheck), 26),
			m.Name,
			fwStr)
	}
	return sb.String()
}

func formatMonitorsFailing(r monitorsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  count=%d\n\n", output.Bold(output.Red("Failing Monitors")), r.Showing)
	for _, m := range r.Monitors {
		fmt.Fprintf(&sb, "  %s  %s\n",
			output.Col(fmt.Sprint(m.ID), 8),
			m.Name)
		if len(m.Controls) > 0 {
			fmt.Fprintf(&sb, "       controls: %s\n", output.Cyan(controlCodesWithFrameworks(m.Controls)))
		}
		fws := uniqueFrameworks(m.Controls)
		if len(fws) > 0 {
			fmt.Fprintf(&sb, "       frameworks: %s\n", output.Yellow(frameworkDisplay(fws)))
		}
	}
	return sb.String()
}

func compactFinding(v any) any {
	switch f := v.(type) {
	case Finding:
		return map[string]any{
			"id":           f.ID,
			"resourceName": f.ResourceName,
			"displayName":  f.DisplayName,
			"provider":     f.ProviderName,
			"accountId":    f.AccountID,
			"clientAlias":  f.ClientAlias,
		}
	case findingsResult:
		compact := make([]any, len(f.Findings))
		for i, finding := range f.Findings {
			compact[i] = compactFinding(finding)
		}
		return map[string]any{"monitorId": f.MonitorID, "monitorName": f.MonitorName, "total": f.Total, "showing": f.Showing, "findings": compact}
	}
	return v
}

func compactHistory(v any) any {
	switch h := v.(type) {
	case connectionResult:
		return map[string]any{
			"source":      h.Source,
			"clientAlias": h.ClientAlias,
			"status":      h.Status,
			"passed":      h.Passed,
			"failed":      h.Failed,
			"excluded":    h.Excluded,
		}
	case historyResult:
		compact := make([]any, len(h.Connections))
		for i, cr := range h.Connections {
			compact[i] = compactHistory(cr)
		}
		return map[string]any{
			"monitorId":   h.MonitorID,
			"monitorName": h.MonitorName,
			"connections": compact,
		}
	}
	return v
}

func formatFindings(r findingsResult) string {
	var sb strings.Builder
	header := r.MonitorName
	if header == "" {
		header = fmt.Sprintf("Monitor %d", r.MonitorID)
	}
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n",
		output.Bold("Findings: "+header), r.Total, r.Showing)
	if len(r.Controls) > 0 {
		fmt.Fprintf(&sb, "Controls: %s\n", output.Cyan(controlCodesWithFrameworks(r.Controls)))
	}
	if len(r.Frameworks) > 0 {
		fmt.Fprintf(&sb, "Frameworks: %s\n", output.Yellow(frameworkDisplay(r.Frameworks)))
	}
	fmt.Fprintln(&sb)

	if len(r.Findings) == 0 {
		fmt.Fprintf(&sb, "  %s\n", output.Green("No findings — all resources passing."))
		return sb.String()
	}

	for _, f := range r.Findings {
		name := f.DisplayName
		if name == "" {
			name = f.ResourceName
		}
		if name == "" {
			name = f.ID
		}
		fmt.Fprintf(&sb, "  %s  %s\n",
			output.Col(output.Red("FAILED"), 22),
			name)

		var details []string
		if f.ProviderName != "" {
			details = append(details, "provider="+f.ProviderName)
		}
		if f.ClientAlias != "" {
			details = append(details, "account="+f.ClientAlias)
		} else if f.AccountID != "" {
			details = append(details, "account="+f.AccountID)
		}
		if f.ResourceArn != "" {
			details = append(details, "arn="+f.ResourceArn)
		}
		if len(details) > 0 {
			fmt.Fprintf(&sb, "       %s\n", output.Dim(strings.Join(details, "  ")))
		}
		if f.FailingMessage != "" {
			fmt.Fprintf(&sb, "       %s\n", output.Dim(f.FailingMessage))
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
	fmt.Fprintf(&sb, "%s  connections=%d\n",
		output.Bold("Check Details: "+header), len(r.Connections))
	if len(r.Controls) > 0 {
		fmt.Fprintf(&sb, "Controls: %s\n", output.Cyan(controlCodesWithFrameworks(r.Controls)))
	}
	if len(r.Frameworks) > 0 {
		fmt.Fprintf(&sb, "Frameworks: %s\n", output.Yellow(frameworkDisplay(r.Frameworks)))
	}
	fmt.Fprintln(&sb)

	if len(r.Connections) == 0 {
		fmt.Fprintf(&sb, "  %s\n", output.Dim("No check details available."))
		return sb.String()
	}

	for _, cr := range r.Connections {
		alias := cr.ClientAlias
		if alias == "" {
			alias = cr.ClientID
		}
		fmt.Fprintf(&sb, "  %s  %s  %s  passed=%s  failed=%s",
			output.Col(output.StatusColor(cr.Status), 22),
			output.Col(cr.Source, 6),
			output.Col(alias, 30),
			output.Green(fmt.Sprint(cr.Passed)),
			output.Red(fmt.Sprint(cr.Failed)))
		if cr.Excluded > 0 {
			fmt.Fprintf(&sb, "  excluded=%s", output.Dim(fmt.Sprint(cr.Excluded)))
		}
		fmt.Fprintln(&sb)
	}
	return sb.String()
}

func formatRetest(r retestResult) string {
	return fmt.Sprintf("%s  Retest triggered for %s [%d]\n",
		output.Green("OK"), r.MonitorName, r.MonitorID)
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
		fmt.Fprintf(&sb, "\nControls: %s\n", output.Cyan(controlCodesWithFrameworks(m.Controls)))
	}
	fws := uniqueFrameworks(m.Controls)
	if len(fws) > 0 {
		fmt.Fprintf(&sb, "Frameworks: %s\n", output.Yellow(frameworkDisplay(fws)))
	}
	return sb.String()
}
