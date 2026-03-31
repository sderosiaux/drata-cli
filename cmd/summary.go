package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type summaryResult struct {
	Status     string `json:"status"`
	Controls   struct {
		Total         int `json:"total"`
		Passing       int `json:"passing"`
		NeedsAttention int `json:"needs_attention"`
	} `json:"controls"`
	Monitors struct {
		Total   int `json:"total"`
		Passing int `json:"passing"`
		Failed  int `json:"failed"`
	} `json:"monitors"`
	Personnel struct {
		Total       int `json:"total"`
		WithIssues  int `json:"with_issues"`
	} `json:"personnel"`
	Connections struct {
		Total      int `json:"total"`
		Connected  int `json:"connected"`
		Failed     int `json:"failed"`
	} `json:"connections"`
	Recommendation string `json:"recommendation,omitempty"`
}

func summaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Overall compliance dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			// GetPage (single page, no auto-pagination) for a fast dashboard.
			// Totals may be approximate when > 50 items exist.
			var (
				ctrlItems, monItems, persItems, connItems []json.RawMessage
				ctrlTotal, monTotal                       int
				ctrlErr, monErr, persErr, connErr         error
			)
			var wg sync.WaitGroup
			wg.Add(4)
			go func() {
				defer wg.Done()
				ctrlItems, ctrlTotal, ctrlErr = c.GetPage("/public/controls", nil)
			}()
			go func() {
				defer wg.Done()
				monItems, monTotal, monErr = c.GetPage("/public/monitors", nil)
			}()
			go func() {
				defer wg.Done()
				persItems, _, persErr = c.GetPage("/public/personnel", nil)
			}()
			go func() {
				defer wg.Done()
				connItems, _, connErr = c.GetPage("/public/connections", nil)
			}()
			wg.Wait()
			_ = ctrlTotal
			_ = monTotal

			for _, err := range []error{ctrlErr, monErr, persErr, connErr} {
				if err != nil {
					return err
				}
			}

			var result summaryResult

			// Controls
			result.Controls.Total = len(ctrlItems)
			for _, raw := range ctrlItems {
				var ctrl Control
				if err := json.Unmarshal(raw, &ctrl); err != nil {
					continue
				}
				e := enrich(ctrl)
				if e.Status == "PASSING" {
					result.Controls.Passing++
				} else if e.Status != "ARCHIVED" {
					result.Controls.NeedsAttention++
				}
			}

			// Monitors
			result.Monitors.Total = len(monItems)
			for _, raw := range monItems {
				var m Monitor
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				switch m.CheckResultStatus {
				case "PASSED":
					result.Monitors.Passing++
				case "FAILED":
					result.Monitors.Failed++
				}
			}

			// Personnel
			result.Personnel.Total = len(persItems)
			for _, raw := range persItems {
				var p Personnel
				if err := json.Unmarshal(raw, &p); err != nil {
					continue
				}
				if p.DevicesFailingComplianceCount > 0 {
					result.Personnel.WithIssues++
				}
			}

			// Connections
			result.Connections.Total = len(connItems)
			for _, raw := range connItems {
				var conn Connection
				if err := json.Unmarshal(raw, &conn); err != nil {
					continue
				}
				if conn.Connected {
					result.Connections.Connected++
				} else if conn.FailedAt != nil {
					result.Connections.Failed++
				}
			}

			// Overall status
			hasIssues := result.Controls.NeedsAttention > 0 ||
				result.Monitors.Failed > 0 ||
				result.Personnel.WithIssues > 0 ||
				result.Connections.Failed > 0
			if hasIssues {
				result.Status = "NEEDS_ATTENTION"
				result.Recommendation = buildRecommendation(result)
			} else {
				result.Status = "COMPLIANT"
			}

			output.Print(result, formatSummary(result), compactSummary)
			return nil
		},
	}
}

func buildRecommendation(r summaryResult) string {
	var parts []string
	if r.Controls.NeedsAttention > 0 {
		parts = append(parts, fmt.Sprintf("fix %d control(s)", r.Controls.NeedsAttention))
	}
	if r.Monitors.Failed > 0 {
		parts = append(parts, fmt.Sprintf("investigate %d failing monitor(s)", r.Monitors.Failed))
	}
	if r.Personnel.WithIssues > 0 {
		parts = append(parts, fmt.Sprintf("resolve device issues for %d employee(s)", r.Personnel.WithIssues))
	}
	if r.Connections.Failed > 0 {
		parts = append(parts, fmt.Sprintf("reconnect %d failed integration(s)", r.Connections.Failed))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Action needed: " + strings.Join(parts, "; ")
}

func compactSummary(v any) any {
	if r, ok := v.(summaryResult); ok {
		return map[string]any{
			"status":     r.Status,
			"controls":   r.Controls,
			"monitors":   r.Monitors,
			"personnel":  r.Personnel,
			"connections": r.Connections,
		}
	}
	return v
}

func formatSummary(r summaryResult) string {
	var sb strings.Builder

	statusLine := output.Green("COMPLIANT")
	if r.Status == "NEEDS_ATTENTION" {
		statusLine = output.Red("NEEDS_ATTENTION")
	}
	sb.WriteString(fmt.Sprintf("%s  %s\n\n", output.Bold("Compliance Summary"), statusLine))

	// Controls
	ctrlStatus := output.Green(fmt.Sprint(r.Controls.Passing))
	sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Controls")))
	sb.WriteString(fmt.Sprintf("    total=%d  passing=%s  needs_attention=%s\n\n",
		r.Controls.Total,
		ctrlStatus,
		output.Red(fmt.Sprint(r.Controls.NeedsAttention)),
	))

	// Monitors
	sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Monitors")))
	sb.WriteString(fmt.Sprintf("    total=%d  passing=%s  failed=%s\n\n",
		r.Monitors.Total,
		output.Green(fmt.Sprint(r.Monitors.Passing)),
		output.Red(fmt.Sprint(r.Monitors.Failed)),
	))

	// Personnel
	sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Personnel")))
	sb.WriteString(fmt.Sprintf("    total=%d  with_device_issues=%s\n\n",
		r.Personnel.Total,
		output.Red(fmt.Sprint(r.Personnel.WithIssues)),
	))

	// Connections
	sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Connections")))
	sb.WriteString(fmt.Sprintf("    total=%d  connected=%s  failed=%s\n",
		r.Connections.Total,
		output.Green(fmt.Sprint(r.Connections.Connected)),
		output.Red(fmt.Sprint(r.Connections.Failed)),
	))

	if r.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", output.Yellow(r.Recommendation)))
	}

	return sb.String()
}
