package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sderosiaux/drata-cli/internal/config"
)

const (
	maxRetries  = 3
	pageSize    = 50
	baseDelayMs = 500
)

type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: config.BaseURL(),
		apiKey:  config.APIKey,
	}
}

// Page response envelope from Drata API.
type pageResponse struct {
	Data  json.RawMessage `json:"data"`
	Total int             `json:"total"`
}

// Get fetches a single resource and returns the raw JSON body.
func (c *Client) Get(path string) (json.RawMessage, error) {
	return c.do(path, nil)
}

// GetPage fetches a single page (page=1, limit=50) without auto-pagination.
// Use for dashboards/summaries where approximate counts are sufficient.
func (c *Client) GetPage(path string, params url.Values) ([]json.RawMessage, int, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("limit", strconv.Itoa(pageSize))
	params.Set("page", "1")

	raw, err := c.do(path, params)
	if err != nil {
		return nil, 0, err
	}

	var pr pageResponse
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, 0, fmt.Errorf("parse response: %w", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(pr.Data, &items); err != nil {
		return nil, 0, fmt.Errorf("parse items: %w", err)
	}

	return items, pr.Total, nil
}

// GetAll auto-paginates and returns all items as a slice of raw JSON objects.
func (c *Client) GetAll(path string, params url.Values) ([]json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("limit", strconv.Itoa(pageSize))

	var all []json.RawMessage
	page := 1

	for {
		params.Set("page", strconv.Itoa(page))
		raw, err := c.do(path, params)
		if err != nil {
			return nil, err
		}

		var pr pageResponse
		if err := json.Unmarshal(raw, &pr); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}

		// data may be an array or wrapped differently
		var items []json.RawMessage
		if err := json.Unmarshal(pr.Data, &items); err != nil {
			// not an array — return raw as single item; unmarshal error is intentionally discarded
			return []json.RawMessage{raw}, nil //nolint:nilerr
		}

		all = append(all, items...)

		if len(items) < pageSize || (pr.Total > 0 && len(all) >= pr.Total) {
			break
		}
		page++
	}

	return all, nil
}

func (c *Client) do(path string, params url.Values) (json.RawMessage, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			time.Sleep(time.Duration(baseDelayMs*(1<<(attempt-1))) * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			return body, nil
		}

		// Retry on 429 and 5xx
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = httpError(resp.StatusCode, body)
			continue
		}

		// Fail-fast on 4xx
		return nil, httpError(resp.StatusCode, body)
	}

	return nil, lastErr
}

func httpError(status int, body []byte) error {
	switch status {
	case 401:
		return fmt.Errorf("401 Unauthorized — check DRATA_API_KEY")
	case 403:
		return fmt.Errorf("403 Forbidden — token lacks permission for this resource")
	case 404:
		return fmt.Errorf("404 Not Found — resource does not exist")
	case 429:
		return fmt.Errorf("429 Rate Limited — too many requests")
	default:
		var msg struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(body, &msg); err == nil && (msg.Message != "" || msg.Error != "") {
			if msg.Message != "" {
				return fmt.Errorf("%d %s", status, msg.Message)
			}
			return fmt.Errorf("%d %s", status, msg.Error)
		}
		return fmt.Errorf("HTTP %d", status)
	}
}
