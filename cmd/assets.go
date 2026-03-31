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

type AssetOwner struct {
	Email string `json:"email"`
}

type Asset struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	AssetType     string     `json:"assetType"`
	AssetProvider string     `json:"assetProvider"`
	Owner         AssetOwner `json:"owner"`
	Company       string     `json:"company"`
}

type assetsResult struct {
	Total   int     `json:"total"`
	Showing int     `json:"showing"`
	Assets  []Asset `json:"assets"`
}

func assetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Manage assets",
	}

	var typeFlag string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New()
			params := url.Values{}
			if typeFlag != "" {
				params.Set("assetType", typeFlag)
			}
			items, err := c.GetAll("/public/assets", params)
			if err != nil {
				return err
			}

			var result assetsResult
			for _, raw := range items {
				var a Asset
				if err := json.Unmarshal(raw, &a); err != nil {
					continue
				}
				result.Assets = append(result.Assets, a)
			}
			result.Total = len(items)
			result.Assets = output.LimitSlice(result.Assets)
			result.Showing = len(result.Assets)

			output.Print(result, formatAssets(result), compactAsset)
			return nil
		},
	}
	listCmd.Flags().StringVar(&typeFlag, "type", "", "Filter: PHYSICAL, VIRTUAL, CLOUD, DATA, PERSONNEL")

	cmd.AddCommand(listCmd)
	return cmd
}

func compactAsset(v any) any {
	switch a := v.(type) {
	case Asset:
		return map[string]any{
			"id":        a.ID,
			"name":      a.Name,
			"assetType": a.AssetType,
			"owner":     a.Owner.Email,
		}
	case assetsResult:
		compact := make([]any, len(a.Assets))
		for i, asset := range a.Assets {
			compact[i] = compactAsset(asset)
		}
		return map[string]any{"total": a.Total, "showing": a.Showing, "assets": compact}
	}
	return v
}

func formatAssets(r assetsResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  total=%d  showing=%d\n\n",
		output.Bold("Assets"), r.Total, r.Showing)
	for _, a := range r.Assets {
		provider := a.AssetProvider
		if provider == "" {
			provider = "-"
		}
		fmt.Fprintf(&sb, "  %s  %s  %s  %s\n",
			output.Col(fmt.Sprint(a.ID), 8),
			output.Col(output.Cyan(a.AssetType), 14),
			output.Col(a.Owner.Email, 32),
			a.Name)
		if provider != "-" {
			fmt.Fprintf(&sb, "       provider=%s\n", output.Dim(provider))
		}
	}
	return sb.String()
}
