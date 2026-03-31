package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type Vendor struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Website   string `json:"website"`
	Status    string `json:"status"`
	RiskLevel string `json:"riskLevel"`
	Category  string `json:"category"`
}

type vendorsResult struct {
	Total   int      `json:"total"`
	Showing int      `json:"showing"`
	Vendors []Vendor `json:"vendors"`
}

func vendorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendors",
		Short: "Manage vendors",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all vendors",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/vendors", nil)
			if err != nil {
				return err
			}

			var result vendorsResult
			for _, raw := range items {
				var v Vendor
				if err := json.Unmarshal(raw, &v); err != nil {
					continue
				}
				result.Vendors = append(result.Vendors, v)
			}
			result.Total = len(items)
			result.Vendors = output.LimitSlice(result.Vendors)
			result.Showing = len(result.Vendors)

			output.Print(result, formatVendors(result), compactVendor)
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get vendor details by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			raw, err := c.Get("/public/vendors/" + args[0])
			if err != nil {
				return err
			}
			var v Vendor
			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("parse vendor: %w", err)
			}
			output.Print(v, formatVendorDetail(v), compactVendor)
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd)
	return cmd
}

func compactVendor(v any) any {
	switch vv := v.(type) {
	case Vendor:
		return map[string]any{
			"id":        vv.ID,
			"name":      vv.Name,
			"status":    vv.Status,
			"riskLevel": vv.RiskLevel,
		}
	case vendorsResult:
		compact := make([]any, len(vv.Vendors))
		for i, vendor := range vv.Vendors {
			compact[i] = compactVendor(vendor)
		}
		return map[string]any{"total": vv.Total, "showing": vv.Showing, "vendors": compact}
	}
	return v
}

func riskColor(risk string) string {
	switch strings.ToUpper(risk) {
	case "HIGH", "CRITICAL":
		return output.Red(risk)
	case "MEDIUM":
		return output.Yellow(risk)
	case "LOW":
		return output.Green(risk)
	default:
		return output.Dim(risk)
	}
}

func formatVendors(r vendorsResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Vendors"), r.Total, r.Showing))
	for _, v := range r.Vendors {
		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(v.ID), 8),
			output.Col(output.StatusColor(v.Status), 22),
			output.Col(riskColor(v.RiskLevel), 16),
			v.Name,
		))
	}
	return sb.String()
}

func formatVendorDetail(v Vendor) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  [%d]\n", output.Bold(v.Name), v.ID))
	sb.WriteString(fmt.Sprintf("Status:   %s\n", output.StatusColor(v.Status)))
	sb.WriteString(fmt.Sprintf("Risk:     %s\n", riskColor(v.RiskLevel)))
	sb.WriteString(fmt.Sprintf("Category: %s\n", v.Category))
	if v.Website != "" {
		sb.WriteString(fmt.Sprintf("Website:  %s\n", output.Cyan(v.Website)))
	}
	return sb.String()
}
