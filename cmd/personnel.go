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

type PersonnelUser struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type Personnel struct {
	ID                           int           `json:"id"`
	User                         PersonnelUser `json:"user"`
	EmploymentStatus             string        `json:"employmentStatus"`
	StartDate                    *string       `json:"startDate"`
	DevicesCount                 int           `json:"devicesCount"`
	DevicesFailingComplianceCount int          `json:"devicesFailingComplianceCount"`
}

type personnelResult struct {
	Total     int         `json:"total"`
	Showing   int         `json:"showing"`
	Personnel []Personnel `json:"personnel"`
}

func personnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "personnel",
		Short: "Manage personnel",
	}

	var statusFlag string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all personnel",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{}
			if statusFlag != "" {
				params.Set("employmentStatus", statusFlag)
			}
			items, err := c.GetAll("/public/personnel", params)
			if err != nil {
				return err
			}

			var result personnelResult
			for _, raw := range items {
				var p Personnel
				if err := json.Unmarshal(raw, &p); err != nil {
					continue
				}
				result.Personnel = append(result.Personnel, p)
			}
			result.Total = len(items)
			result.Showing = len(result.Personnel)

			output.Print(result, formatPersonnel(result), compactPersonnel)
			return nil
		},
	}
	listCmd.Flags().StringVar(&statusFlag, "status", "", "Filter: CURRENT_EMPLOYEE, CURRENT_CONTRACTOR, FORMER_EMPLOYEE, FORMER_CONTRACTOR")

	issuesCmd := &cobra.Command{
		Use:   "issues",
		Short: "List personnel with device compliance issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/personnel", nil)
			if err != nil {
				return err
			}

			var result personnelResult
			for _, raw := range items {
				var p Personnel
				if err := json.Unmarshal(raw, &p); err != nil {
					continue
				}
				if p.DevicesFailingComplianceCount > 0 {
					result.Personnel = append(result.Personnel, p)
				}
			}
			result.Total = len(items)
			result.Showing = len(result.Personnel)

			output.Print(result, formatPersonnelIssues(result), compactPersonnel)
			return nil
		},
	}

	var emailFlag string
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get personnel details by ID or --email",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()

			if emailFlag != "" {
				// API doesn't support email filtering — fetch all and match client-side
				items, err := c.GetAll("/public/personnel", nil)
				if err != nil {
					return err
				}
				for _, raw := range items {
					var p Personnel
					if err := json.Unmarshal(raw, &p); err != nil {
						continue
					}
					if strings.EqualFold(p.User.Email, emailFlag) {
						output.Print(p, formatPersonnelDetail(p), compactPersonnel)
						return nil
					}
				}
				return fmt.Errorf("no personnel found with email %q", emailFlag)
			}

			if len(args) == 0 {
				return fmt.Errorf("provide an ID or --email flag")
			}
			raw, err := c.Get("/public/personnel/" + args[0])
			if err != nil {
				return err
			}
			var p Personnel
			if err := json.Unmarshal(raw, &p); err != nil {
				return fmt.Errorf("parse personnel: %w", err)
			}
			output.Print(p, formatPersonnelDetail(p), compactPersonnel)
			return nil
		},
	}
	getCmd.Flags().StringVar(&emailFlag, "email", "", "Find personnel by email address")

	cmd.AddCommand(listCmd, issuesCmd, getCmd)
	return cmd
}

func compactPersonnel(v any) any {
	switch p := v.(type) {
	case Personnel:
		return map[string]any{
			"id":     p.ID,
			"email":  p.User.Email,
			"status": p.EmploymentStatus,
			"failing_devices": p.DevicesFailingComplianceCount,
		}
	case personnelResult:
		compact := make([]any, len(p.Personnel))
		for i, per := range p.Personnel {
			compact[i] = compactPersonnel(per)
		}
		return map[string]any{"total": p.Total, "showing": p.Showing, "personnel": compact}
	}
	return v
}

func formatPersonnel(r personnelResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Personnel"), r.Total, r.Showing))
	for _, p := range r.Personnel {
		name := strings.TrimSpace(p.User.FirstName + " " + p.User.LastName)
		failStr := ""
		if p.DevicesFailingComplianceCount > 0 {
			failStr = output.Red(fmt.Sprintf(" [%d failing]", p.DevicesFailingComplianceCount))
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %s%s\n",
			output.Col(fmt.Sprint(p.ID), 8),
			output.Col(output.StatusColor(p.EmploymentStatus), 28),
			output.Col(p.User.Email, 36),
			name+failStr,
		))
	}
	return sb.String()
}

func formatPersonnelIssues(r personnelResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  count=%d\n\n", output.Bold(output.Red("Personnel with Device Issues")), r.Showing))
	for _, p := range r.Personnel {
		name := strings.TrimSpace(p.User.FirstName + " " + p.User.LastName)
		sb.WriteString(fmt.Sprintf("  %s  %s  devices=%d  failing=%s\n",
			output.Col(fmt.Sprint(p.ID), 8),
			output.Col(p.User.Email, 36),
			p.DevicesCount,
			output.Red(fmt.Sprint(p.DevicesFailingComplianceCount)),
		))
		if name != "" {
			sb.WriteString(fmt.Sprintf("       name: %s\n", name))
		}
	}
	return sb.String()
}

func formatPersonnelDetail(p Personnel) string {
	name := strings.TrimSpace(p.User.FirstName + " " + p.User.LastName)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  [%d]\n", output.Bold(p.User.Email), p.ID))
	if name != "" {
		sb.WriteString(fmt.Sprintf("Name:   %s\n", name))
	}
	sb.WriteString(fmt.Sprintf("Status: %s\n", output.StatusColor(p.EmploymentStatus)))
	if p.StartDate != nil {
		sb.WriteString(fmt.Sprintf("Start:  %s\n", *p.StartDate))
	}
	sb.WriteString(fmt.Sprintf("Devices: %d total, %s failing compliance\n",
		p.DevicesCount,
		output.Red(fmt.Sprint(p.DevicesFailingComplianceCount)),
	))
	return sb.String()
}
