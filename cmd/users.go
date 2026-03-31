package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sderosiaux/drata-cli/internal/client"
	"github.com/sderosiaux/drata-cli/internal/output"
)

type User struct {
	ID        int      `json:"id"`
	Email     string   `json:"email"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	JobTitle  string   `json:"jobTitle"`
	Roles     []string `json:"roles"`
	CreatedAt string   `json:"createdAt"`
}

type usersResult struct {
	Total   int    `json:"total"`
	Showing int    `json:"showing"`
	Users   []User `json:"users"`
}

func usersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage Drata users",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			items, err := c.GetAll("/public/users", nil)
			if err != nil {
				return err
			}

			var result usersResult
			for _, raw := range items {
				var u User
				if err := json.Unmarshal(raw, &u); err != nil {
					continue
				}
				result.Users = append(result.Users, u)
			}
			result.Total = len(items)
			result.Users = output.LimitSlice(result.Users)
			result.Showing = len(result.Users)

			output.Print(result, formatUsers(result), compactUser)
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	return cmd
}

func compactUser(v any) any {
	switch u := v.(type) {
	case User:
		return map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"jobTitle": u.JobTitle,
			"roles":    u.Roles,
		}
	case usersResult:
		compact := make([]any, len(u.Users))
		for i, usr := range u.Users {
			compact[i] = compactUser(usr)
		}
		return map[string]any{"total": u.Total, "showing": u.Showing, "users": compact}
	}
	return v
}

func formatUsers(r usersResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s  total=%d  showing=%d\n\n",
		output.Bold("Users"), r.Total, r.Showing))
	for _, u := range r.Users {
		name := strings.TrimSpace(u.FirstName + " " + u.LastName)
		roles := strings.Join(u.Roles, ", ")
		sb.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			output.Col(fmt.Sprint(u.ID), 8),
			output.Col(u.Email, 36),
			name,
		))
		if roles != "" {
			sb.WriteString(fmt.Sprintf("       roles=%s\n", output.Dim(roles)))
		}
	}
	return sb.String()
}
