package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type Policy struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Status      string  `json:"status"`
	UpdatedAt   *string `json:"updatedAt"`
	PublishedAt *string `json:"publishedAt"`
}

type policiesResult struct {
	Total    int      `json:"total"`
	Showing  int      `json:"showing"`
	Policies []Policy `json:"policies"`
}

func policiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policies",
		Short: "Manage compliance policies",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/policies", nil)
			if err != nil {
				return err
			}

			var result policiesResult
			for _, raw := range items {
				var p Policy
				if err := json.Unmarshal(raw, &p); err != nil {
					continue
				}
				result.Policies = append(result.Policies, p)
			}
			result.Total = len(items)
			result.Policies = output.LimitSlice(result.Policies)
			result.Showing = len(result.Policies)

			output.Print(result, formatPolicies(result), compactPolicy)
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get policy details by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			raw, err := c.Get("/public/policies/" + args[0])
			if err != nil {
				return err
			}
			var p Policy
			if err := json.Unmarshal(raw, &p); err != nil {
				return fmt.Errorf("parse policy: %w", err)
			}
			output.Print(p, formatPolicyDetail(p), compactPolicy)
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd)
	return cmd
}

func compactPolicy(v any) any {
	switch p := v.(type) {
	case Policy:
		return map[string]any{"id": p.ID, "name": p.Name, "version": p.Version, "status": p.Status}
	case policiesResult:
		compact := make([]any, len(p.Policies))
		for i, pol := range p.Policies {
			compact[i] = compactPolicy(pol)
		}
		return map[string]any{"total": p.Total, "showing": p.Showing, "policies": compact}
	}
	return v
}

func formatPolicies(r policiesResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n\n",
		output.Bold("Policies"), r.Total, r.Showing)
	for _, p := range r.Policies {
		updated := ""
		if p.UpdatedAt != nil {
			updated = *p.UpdatedAt
		}
		fmt.Fprintf(&sb, "  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(p.ID), 8),
			output.Col(output.StatusColor(p.Status), 22),
			output.Col(output.Dim(updated), 26),
			p.Name)
	}
	return sb.String()
}

func formatPolicyDetail(p Policy) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  [%d]\n", output.Bold(p.Name), p.ID)
	fmt.Fprintf(&sb, "Status:  %s\n", output.StatusColor(p.Status))
	fmt.Fprintf(&sb, "Version: %s\n", p.Version)
	if p.UpdatedAt != nil {
		fmt.Fprintf(&sb, "Updated: %s\n", *p.UpdatedAt)
	}
	if p.PublishedAt != nil {
		fmt.Fprintf(&sb, "Published: %s\n", *p.PublishedAt)
	}
	return sb.String()
}
