package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type DeviceUser struct {
	Email string `json:"email"`
}

type Device struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	SerialNumber string     `json:"serialNumber"`
	Platform     string     `json:"platform"`
	OsVersion    string     `json:"osVersion"`
	User         DeviceUser `json:"user"`
}

type devicesResult struct {
	Total   int      `json:"total"`
	Showing int      `json:"showing"`
	Devices []Device `json:"devices"`
}

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage employee devices",
	}

	var userFlag string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/devices", nil)
			if err != nil {
				return err
			}

			var result devicesResult
			for _, raw := range items {
				var d Device
				if err := json.Unmarshal(raw, &d); err != nil {
					continue
				}
				if userFlag != "" && !strings.EqualFold(d.User.Email, userFlag) {
					continue
				}
				result.Devices = append(result.Devices, d)
			}
			result.Total = len(items)
			result.Devices = output.LimitSlice(result.Devices)
			result.Showing = len(result.Devices)

			output.Print(result, formatDevices(result), compactDevice)
			return nil
		},
	}
	listCmd.Flags().StringVar(&userFlag, "user", "", "Filter by user email")

	cmd.AddCommand(listCmd)
	return cmd
}

func compactDevice(v any) any {
	switch d := v.(type) {
	case Device:
		return map[string]any{
			"id":       d.ID,
			"name":     d.Name,
			"platform": d.Platform,
			"user":     d.User.Email,
		}
	case devicesResult:
		compact := make([]any, len(d.Devices))
		for i, dev := range d.Devices {
			compact[i] = compactDevice(dev)
		}
		return map[string]any{"total": d.Total, "showing": d.Showing, "devices": compact}
	}
	return v
}

func formatDevices(r devicesResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Devices"), r.Total, r.Showing))
	for _, d := range r.Devices {
		osInfo := d.Platform
		if d.OsVersion != "" {
			osInfo += " " + d.OsVersion
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(d.ID), 8),
			output.Col(output.Cyan(d.Platform), 14),
			output.Col(d.User.Email, 36),
			d.Name,
		))
		if d.SerialNumber != "" {
			sb.WriteString(fmt.Sprintf("       serial=%s  os=%s\n",
				output.Dim(d.SerialNumber),
				output.Dim(d.OsVersion),
			))
		}
	}
	return sb.String()
}
