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

// Control represents a Drata compliance control.
type Control struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	IsReady     bool     `json:"isReady"`
	HasOwner    bool     `json:"hasOwner"`
	IsMonitored bool     `json:"isMonitored"`
	HasEvidence bool     `json:"hasEvidence"`
	ArchivedAt  *string  `json:"archivedAt"`
	Frameworks  []string `json:"frameworkTags"`
}

func controlStatus(c Control) string {
	if c.ArchivedAt != nil {
		return "ARCHIVED"
	}
	if !c.IsReady {
		return "NOT_READY"
	}
	if !c.HasOwner {
		return "NO_OWNER"
	}
	if c.IsMonitored && c.HasEvidence {
		return "PASSING"
	}
	if !c.HasEvidence {
		return "NEEDS_EVIDENCE"
	}
	return "READY"
}

type enrichedControl struct {
	ID          int      `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	IsMonitored bool     `json:"isMonitored"`
	HasEvidence bool     `json:"hasEvidence"`
	HasOwner    bool     `json:"hasOwner"`
	Frameworks  []string `json:"frameworks"`
}

func enrich(c Control) enrichedControl {
	return enrichedControl{
		ID:          c.ID,
		Code:        c.Code,
		Name:        c.Name,
		Status:      controlStatus(c),
		IsMonitored: c.IsMonitored,
		HasEvidence: c.HasEvidence,
		HasOwner:    c.HasOwner,
		Frameworks:  c.Frameworks,
	}
}

type controlsResult struct {
	Total   int `json:"total"`
	Showing int `json:"showing"`
	Summary struct {
		Passing       int `json:"passing"`
		NotReady      int `json:"not_ready"`
		NoOwner       int `json:"no_owner"`
		NeedsEvidence int `json:"needs_evidence"`
		Archived      int `json:"archived"`
	} `json:"summary"`
	Controls []enrichedControl `json:"controls"`
}

func controlsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controls",
		Short: "Manage compliance controls",
	}

	// list
	var (
		statusFlag string
		searchFlag string
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all controls with derived compliance status",
		Long: `List all controls. Status is derived by the CLI (not an API field):
  PASSING        monitored and has evidence
  NEEDS_EVIDENCE missing evidence upload
  NOT_READY      control not configured
  NO_OWNER       no owner assigned
  READY          configured but not yet monitored
  ARCHIVED       archived control (excluded from compliance score)`,
		Example: `  drata controls list
  drata controls list --status NO_OWNER
  drata controls list --status NEEDS_EVIDENCE --json --compact
  drata controls list --search "MFA"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{}
			if searchFlag != "" {
				params.Set("q", searchFlag)
			}
			items, err := c.GetAll("/public/controls", params)
			if err != nil {
				return err
			}

			var result controlsResult
			for _, raw := range items {
				var ctrl Control
				if err := json.Unmarshal(raw, &ctrl); err != nil {
					continue
				}
				e := enrich(ctrl)
				if statusFlag != "" && e.Status != statusFlag {
					continue
				}
				result.Controls = append(result.Controls, e)
			}
			result.Total = len(items)
			for _, c := range result.Controls {
				switch c.Status {
				case "PASSING":
					result.Summary.Passing++
				case "NOT_READY":
					result.Summary.NotReady++
				case "NO_OWNER":
					result.Summary.NoOwner++
				case "NEEDS_EVIDENCE":
					result.Summary.NeedsEvidence++
				case "ARCHIVED":
					result.Summary.Archived++
				}
			}
			result.Controls = output.LimitSlice(result.Controls)
			result.Showing = len(result.Controls)

			output.Print(result, formatControls(result), compactControl)
			return nil
		},
	}
	listCmd.Flags().StringVar(&statusFlag, "status", "", "Filter: PASSING, NOT_READY, NO_OWNER, NEEDS_EVIDENCE, ARCHIVED")
	listCmd.Flags().StringVar(&searchFlag, "search", "", "Full-text search on control name or code")

	// failing
	failingCmd := &cobra.Command{
		Use:     "failing",
		Short:   "List controls with issues (NOT_READY, NO_OWNER, NEEDS_EVIDENCE)",
		Example: "  drata controls failing --json --compact",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/controls", nil)
			if err != nil {
				return err
			}

			var result controlsResult
			for _, raw := range items {
				var ctrl Control
				if err := json.Unmarshal(raw, &ctrl); err != nil {
					continue
				}
				e := enrich(ctrl)
				switch e.Status {
				case "NOT_READY", "NO_OWNER", "NEEDS_EVIDENCE":
					result.Controls = append(result.Controls, e)
					switch e.Status {
					case "NOT_READY":
						result.Summary.NotReady++
					case "NO_OWNER":
						result.Summary.NoOwner++
					case "NEEDS_EVIDENCE":
						result.Summary.NeedsEvidence++
					}
				}
			}
			result.Total = len(items)
			result.Controls = output.LimitSlice(result.Controls)
			result.Showing = len(result.Controls)

			output.Print(result, formatControls(result), compactControl)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get <code>",
		Short: "Get control details by code (e.g. DCF-71)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code := args[0]
			c := client.New()

			params := url.Values{}
			params.Set("q", code)
			items, err := c.GetAll("/public/controls", params)
			if err != nil {
				return err
			}

			var found *enrichedControl
			for _, raw := range items {
				var ctrl Control
				if err := json.Unmarshal(raw, &ctrl); err != nil {
					continue
				}
				if ctrl.Code == code {
					e := enrich(ctrl)
					found = &e
					break
				}
			}

			if found == nil {
				return fmt.Errorf("control %q not found", code)
			}

			output.Print(found, formatControl(*found), compactControl)
			return nil
		},
	}

	// evidence — GET /public/controls/{id}/external-evidence
	evidenceSubCmd := &cobra.Command{
		Use:   "evidence <code>",
		Short: "List external evidence for a control (e.g. DCF-71)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code := args[0]
			c := client.New()

			// resolve code → id
			params := url.Values{}
			params.Set("q", code)
			items, err := c.GetAll("/public/controls", params)
			if err != nil {
				return err
			}
			var ctrlID int
			for _, raw := range items {
				var ctrl Control
				if err := json.Unmarshal(raw, &ctrl); err != nil {
					continue
				}
				if ctrl.Code == code {
					ctrlID = ctrl.ID
					break
				}
			}
			if ctrlID == 0 {
				return fmt.Errorf("control %q not found", code)
			}

			raw, err := c.Get(fmt.Sprintf("/public/controls/%d/external-evidence", ctrlID))
			if err != nil {
				return err
			}
			// Pass raw JSON through — evidence structure varies
			var ev any
			if err := json.Unmarshal(raw, &ev); err != nil {
				return fmt.Errorf("parse evidence: %w", err)
			}
			output.Print(ev, fmt.Sprintf("Evidence for %s:\n%s", code, string(raw)), nil)
			return nil
		},
	}

	cmd.AddCommand(listCmd, failingCmd, getCmd, evidenceSubCmd)
	return cmd
}

func compactControl(v any) any {
	switch c := v.(type) {
	case enrichedControl:
		return map[string]any{
			"id":     c.ID,
			"code":   c.Code,
			"name":   c.Name,
			"status": c.Status,
		}
	case *enrichedControl:
		return map[string]any{
			"id":     c.ID,
			"code":   c.Code,
			"name":   c.Name,
			"status": c.Status,
		}
	case controlsResult:
		compact := make([]any, len(c.Controls))
		for i, ctrl := range c.Controls {
			compact[i] = compactControl(ctrl)
		}
		return map[string]any{
			"total":    c.Total,
			"showing":  c.Showing,
			"summary":  c.Summary,
			"controls": compact,
		}
	}
	return v
}

func formatControls(r controlsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n",
		output.Bold("Controls"),
		r.Total, r.Showing)
	fmt.Fprintf(&sb, "  passing=%s  not_ready=%s  no_owner=%s  needs_evidence=%s  archived=%s\n\n",
		output.Green(fmt.Sprint(r.Summary.Passing)),
		output.Red(fmt.Sprint(r.Summary.NotReady)),
		output.Red(fmt.Sprint(r.Summary.NoOwner)),
		output.Yellow(fmt.Sprint(r.Summary.NeedsEvidence)),
		output.Dim(fmt.Sprint(r.Summary.Archived)))

	for _, c := range r.Controls {
		fmt.Fprintf(&sb, "  %s  %s  %s\n",
			output.Col(output.Cyan(c.Code), 12),
			output.Col(output.StatusColor(c.Status), 26),
			c.Name)
	}
	return sb.String()
}

func formatControl(c enrichedControl) string {
	return fmt.Sprintf("%s  %s\n%s\nMonitored: %v  Evidence: %v  Owner: %v\nFrameworks: %s",
		output.Cyan(c.Code),
		output.StatusColor(c.Status),
		output.Bold(c.Name),
		c.IsMonitored, c.HasEvidence, c.HasOwner,
		strings.Join(c.Frameworks, ", "),
	)
}
