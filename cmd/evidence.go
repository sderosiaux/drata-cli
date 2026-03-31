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

type EvidenceVersion struct {
	ID int `json:"id"`
}

type Evidence struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
	Versions    []EvidenceVersion `json:"versions"`
}

type evidenceResult struct {
	Total    int        `json:"total"`
	Showing  int        `json:"showing"`
	Evidence []Evidence `json:"evidence"`
}

func getWorkspaceID(c *client.Client) (int, error) {
	type workspace struct {
		ID int `json:"id"`
	}
	wsItems, err := c.GetAll("/public/workspaces", url.Values{"limit": []string{"1"}})
	if err != nil {
		return 0, fmt.Errorf("fetch workspaces: %w", err)
	}
	if len(wsItems) == 0 {
		return 0, fmt.Errorf("no workspaces found")
	}
	var ws workspace
	if err := json.Unmarshal(wsItems[0], &ws); err != nil {
		return 0, fmt.Errorf("parse workspace: %w", err)
	}
	if ws.ID == 0 {
		return 0, fmt.Errorf("workspace ID is 0")
	}
	return ws.ID, nil
}

func evidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Manage evidence library",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all evidence items",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			items, err := c.GetAll(fmt.Sprintf("/public/workspaces/%d/evidence-library", wsID), nil)
			if err != nil {
				return err
			}

			var result evidenceResult
			for _, raw := range items {
				var e Evidence
				if err := json.Unmarshal(raw, &e); err != nil {
					continue
				}
				result.Evidence = append(result.Evidence, e)
			}
			result.Total = len(items)
			result.Showing = len(result.Evidence)

			output.Print(result, formatEvidence(result), compactEvidence)
			return nil
		},
	}

	var daysFlag int
	expiringCmd := &cobra.Command{
		Use:   "expiring",
		Short: "List evidence not updated recently",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			wsID, err := getWorkspaceID(c)
			if err != nil {
				return err
			}

			items, err := c.GetAll(fmt.Sprintf("/public/workspaces/%d/evidence-library", wsID), nil)
			if err != nil {
				return err
			}

			threshold := time.Now().AddDate(0, 0, -daysFlag)
			var result evidenceResult
			for _, raw := range items {
				var e Evidence
				if err := json.Unmarshal(raw, &e); err != nil {
					continue
				}
				if e.UpdatedAt == "" {
					result.Evidence = append(result.Evidence, e)
					continue
				}
				t, err := time.Parse(time.RFC3339, e.UpdatedAt)
				if err != nil {
					// try other common formats
					t, err = time.Parse("2006-01-02T15:04:05.000Z", e.UpdatedAt)
					if err != nil {
						continue
					}
				}
				if t.Before(threshold) {
					result.Evidence = append(result.Evidence, e)
				}
			}
			result.Total = len(items)
			result.Showing = len(result.Evidence)

			output.Print(result, formatEvidenceExpiring(result, daysFlag), compactEvidence)
			return nil
		},
	}
	expiringCmd.Flags().IntVar(&daysFlag, "days", 30, "Evidence not updated in this many days")

	cmd.AddCommand(listCmd, expiringCmd)
	return cmd
}

func compactEvidence(v any) any {
	switch e := v.(type) {
	case Evidence:
		return map[string]any{
			"id":        e.ID,
			"name":      e.Name,
			"updatedAt": e.UpdatedAt,
			"versions":  len(e.Versions),
		}
	case evidenceResult:
		compact := make([]any, len(e.Evidence))
		for i, ev := range e.Evidence {
			compact[i] = compactEvidence(ev)
		}
		return map[string]any{"total": e.Total, "showing": e.Showing, "evidence": compact}
	}
	return v
}

func daysSince(ts string) float64 {
	if ts == "" {
		return -1
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return -1
		}
	}
	return time.Since(t).Hours() / 24
}

func formatEvidence(r evidenceResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n\n",
		output.Bold("Evidence Library"), r.Total, r.Showing)
	for _, e := range r.Evidence {
		days := daysSince(e.UpdatedAt)
		daysStr := ""
		if days >= 0 {
			daysStr = fmt.Sprintf("%.0fd ago", days)
		}
		fmt.Fprintf(&sb, "  %s  %s  versions=%d  %s\n",
			output.Col(fmt.Sprint(e.ID), 8),
			output.Col(e.Name, 40),
			len(e.Versions),
			output.Dim(daysStr))
	}
	return sb.String()
}

func formatEvidenceExpiring(r evidenceResult, days int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  (not updated in >%d days)  count=%d\n\n",
		output.Bold(output.Yellow("Stale Evidence")), days, r.Showing)
	for _, e := range r.Evidence {
		d := daysSince(e.UpdatedAt)
		daysStr := "unknown"
		if d >= 0 {
			daysStr = fmt.Sprintf("%.0f days ago", d)
		}
		fmt.Fprintf(&sb, "  %s  %s  %s\n",
			output.Col(fmt.Sprint(e.ID), 8),
			output.Col(e.Name, 40),
			output.Yellow(daysStr))
	}
	return sb.String()
}
