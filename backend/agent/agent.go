package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Request struct {
	Query     string
	Documents []string
}

type Agent struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func New(apiKey, baseURL, model string) *Agent {
	return &Agent{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{},
	}
}

const systemPrompt = "You are a research assistant. Answer the user's question using the provided source documents. Respond in Markdown. If the documents don't contain the answer, say so clearly."

// parse documents and query to the LLM and use emit to return answers
func (a *Agent) Answer(ctx context.Context, req Request, emit func(string) error) error {
	body := map[string]any{
		"model":  a.model,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildPrompt(req)},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm returned %d: %s", resp.StatusCode, msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 允许较长的行
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := emit(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func buildPrompt(req Request) string {
	var b strings.Builder
	if len(req.Documents) > 0 {
		b.WriteString("Here are the source documents:\n\n")
		for i, doc := range req.Documents {
			fmt.Fprintf(&b, "--- Document %d ---\n%s\n\n", i+1, doc)
		}
	}
	b.WriteString("Question: ")
	b.WriteString(req.Query)
	return b.String()
}
