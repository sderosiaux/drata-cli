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

type EventUser struct {
	Email string `json:"email"`
}

type Event struct {
	ID          int       `json:"id"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	CreatedAt   string    `json:"createdAt"`
	User        EventUser `json:"user"`
}

type eventsResult struct {
	Total   int     `json:"total"`
	Showing int     `json:"showing"`
	Events  []Event `json:"events"`
}

func eventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "View audit events",
	}

	var (
		categoryFlag string
		limitFlag    int
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List audit events",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{}
			if categoryFlag != "" {
				params.Set("category", categoryFlag)
			}
			if limitFlag > 0 {
				params.Set("limit", fmt.Sprint(limitFlag))
			}

			items, err := c.GetAll("/public/events", params)
			if err != nil {
				return err
			}

			var result eventsResult
			for _, raw := range items {
				var e Event
				if err := json.Unmarshal(raw, &e); err != nil {
					continue
				}
				result.Events = append(result.Events, e)
			}
			result.Total = len(items)
			result.Showing = len(result.Events)

			output.Print(result, formatEvents(result), compactEvent)
			return nil
		},
	}
	listCmd.Flags().StringVar(&categoryFlag, "category", "", "Filter: PERSONNEL, CONTROL, MONITOR, CONNECTION, POLICY, VENDOR")
	listCmd.Flags().IntVar(&limitFlag, "limit", 50, "Max events to return")

	cmd.AddCommand(listCmd)
	return cmd
}

func compactEvent(v any) any {
	switch e := v.(type) {
	case Event:
		return map[string]any{
			"id":          e.ID,
			"type":        e.Type,
			"category":    e.Category,
			"description": e.Description,
			"createdAt":   e.CreatedAt,
		}
	case eventsResult:
		compact := make([]any, len(e.Events))
		for i, ev := range e.Events {
			compact[i] = compactEvent(ev)
		}
		return map[string]any{"total": e.Total, "showing": e.Showing, "events": compact}
	}
	return v
}

func formatEvents(r eventsResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Events"), r.Total, r.Showing))
	for _, e := range r.Events {
		userStr := ""
		if e.User.Email != "" {
			userStr = output.Dim(e.User.Email)
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			output.Col(output.Cyan(e.Category), 16),
			output.Col(output.Dim(e.CreatedAt), 28),
			output.Col(userStr, 30),
			e.Description,
		))
	}
	return sb.String()
}
