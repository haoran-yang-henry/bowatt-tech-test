package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// define external information source structure
type Source struct {
	Title   string
	URL     string
	Content string
}

// webSearch using Tavily, return the result with content
func webSearch(ctx context.Context, apiKey, query string) ([]Source, error) {
	body, _ := json.Marshal(map[string]any{
		"api_key":     apiKey,
		"query":       query,
		"max_results": 5,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned %d: %s", resp.StatusCode, msg)
	}

	var out struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	sources := make([]Source, 0, len(out.Results))
	for _, r := range out.Results {
		sources = append(sources, Source{Title: r.Title, URL: r.URL, Content: r.Content})
	}
	return sources, nil
}
