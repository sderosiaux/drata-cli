package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type ProviderType struct {
	Value string `json:"value"`
}

type Connection struct {
	ID            int            `json:"id"`
	ClientType    string         `json:"clientType"`
	State         string         `json:"state"`
	Connected     bool           `json:"connected"`
	ConnectedAt   *string        `json:"connectedAt"`
	FailedAt      *string        `json:"failedAt"`
	ProviderTypes []ProviderType `json:"providerTypes"`
}

type connectionsResult struct {
	Total       int          `json:"total"`
	Showing     int          `json:"showing"`
	Connections []Connection `json:"connections"`
}

func connectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "Manage integrations and connections",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/connections", nil)
			if err != nil {
				return err
			}

			var result connectionsResult
			for _, raw := range items {
				var conn Connection
				if err := json.Unmarshal(raw, &conn); err != nil {
					continue
				}
				result.Connections = append(result.Connections, conn)
			}
			result.Total = len(items)
			result.Showing = len(result.Connections)

			output.Print(result, formatConnections(result), compactConnection)
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	return cmd
}

func compactConnection(v any) any {
	switch c := v.(type) {
	case Connection:
		return map[string]any{
			"id":         c.ID,
			"clientType": c.ClientType,
			"state":      c.State,
			"connected":  c.Connected,
		}
	case connectionsResult:
		compact := make([]any, len(c.Connections))
		for i, conn := range c.Connections {
			compact[i] = compactConnection(conn)
		}
		return map[string]any{"total": c.Total, "showing": c.Showing, "connections": compact}
	}
	return v
}

func formatConnections(r connectionsResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Connections"), r.Total, r.Showing))
	for _, c := range r.Connections {
		connStatus := "DISCONNECTED"
		connTime := ""
		if c.Connected {
			connStatus = "CONNECTED"
			if c.ConnectedAt != nil {
				connTime = *c.ConnectedAt
			}
		} else if c.FailedAt != nil {
			connStatus = "FAILED"
			connTime = *c.FailedAt
		}

		providers := make([]string, len(c.ProviderTypes))
		for i, p := range c.ProviderTypes {
			providers[i] = p.Value
		}
		provStr := strings.Join(providers, ", ")

		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(c.ID), 8),
			output.Col(output.StatusColor(connStatus), 22),
			output.Col(c.ClientType, 28),
			provStr,
		))
		if connTime != "" {
			sb.WriteString(fmt.Sprintf("       %s\n", output.Dim(connTime)))
		}
	}
	return sb.String()
}
