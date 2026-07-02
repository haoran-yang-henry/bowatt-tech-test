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
	apiKey    string
	baseURL   string
	model     string
	searchKey string
	maxRounds int
	http      *http.Client
}

func New(apiKey, baseURL, model, searchKey string) *Agent {
	return &Agent{
		apiKey:    apiKey,
		baseURL:   baseURL,
		model:     model,
		searchKey: searchKey,
		maxRounds: 3, // 最多 3 轮
		http:      &http.Client{},
	}
}

func (a *Agent) deriveFocus(ctx context.Context, req Request) (string, error) {
	var b strings.Builder
	b.WriteString("Documents:\n")
	b.WriteString(joinDocs(req.Documents))
	b.WriteString("\nUser question: " + req.Query + "\n\n")
	b.WriteString("In ONE sentence, state precisely what this research is about, grounded strictly in the documents and the question. Do not introduce any topic not present in the documents.")
	messages := []map[string]string{{"role": "user", "content": b.String()}}
	return a.complete(ctx, messages, false)
}

// main orchastration: step 1 plan, step 2 search loop, step 3 summarize result from search result

func (a *Agent) Answer(ctx context.Context, req Request, emit func(string) error) error {
	focus, err := a.deriveFocus(ctx, req)
	if err != nil {
		return err
	}
	emit("🎯 research focus: " + focus + "\n\n")

	var notes []string
	for round := 1; round <= a.maxRounds; round++ {
		queries, enough, err := a.decideSearch(ctx, focus, notes)
		if err != nil {
			return err
		}
		if enough || len(queries) == 0 {
			break
		}

		emit(fmt.Sprintf("🔎 Research round %d  \n", round))
		var results []Source
		for _, q := range queries {
			emit("  · Search: " + q + "\n")
			found, err := webSearch(ctx, a.searchKey, q)
			if err != nil {
				emit("    (Search failed: " + err.Error() + ")\n")
				continue
			}
			results = append(results, found...)
			if len(results) >= 20 {
				results = results[:20]
				break
			}
		}
		if len(results) == 0 {
			continue
		}

		emit("  · summarying key points...\n")
		summary, err := a.summarizeRound(ctx, focus, results)
		if err != nil {
			return err
		}
		notes = append(notes, summary)
	}

	emit("\n📝 summarizing research result...\n\n")
	return a.synthesize(ctx, req, focus, notes, emit)
}

// planing and reflecting functions
func (a *Agent) decideSearch(ctx context.Context, focus string, notes []string) ([]string, bool, error) {
	var b strings.Builder
	b.WriteString("Research focus (do NOT deviate from this): " + focus + "\n\n")
	if len(notes) > 0 {
		b.WriteString("Notes gathered so far:\n" + strings.Join(notes, "\n") + "\n\n")
	}
	b.WriteString(`Every search query MUST stay strictly on the research focus above. ` +
		`Ignore any earlier note that drifted away from the focus. ` +
		`If the notes already cover the focus, or no on-focus web search would help, reply {"enough": true}. ` +
		`Otherwise reply {"enough": false, "queries": ["..."]} with up to 3 queries directly about the focus. ` +
		`Reply in JSON only.`)

	messages := []map[string]string{{"role": "user", "content": b.String()}}
	content, err := a.complete(ctx, messages, true)
	if err != nil {
		return nil, false, err
	}
	var out struct {
		Enough  bool     `json:"enough"`
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, true, nil
	}
	return out.Queries, out.Enough, nil
}

// context control: transform the search result into key points

func (a *Agent) summarizeRound(ctx context.Context, focus string, results []Source) (string, error) {
	var b strings.Builder
	b.WriteString("Research focus: " + focus + "\n\nSearch results:\n")
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s (%s)\n%s\n\n", i+1, r.Title, r.URL, r.Content)
	}
	b.WriteString("Keep ONLY points relevant to the research focus. Discard any off-topic result. " +
		"If nothing is relevant, reply exactly: No relevant information found. " +
		"Otherwise give concise bullet points, each with its source URL.")

	messages := []map[string]string{{"role": "user", "content": b.String()}}
	return a.complete(ctx, messages, false)
}

// stream output of the final investigation result

func (a *Agent) synthesize(ctx context.Context, req Request, focus string, notes []string, emit func(string) error) error {
	var b strings.Builder
	b.WriteString("Research focus: " + focus + "\n\n")
	b.WriteString("Internal knowledge (documents):\n" + joinDocs(req.Documents))
	if len(notes) > 0 {
		b.WriteString("\nWeb findings:\n" + strings.Join(notes, "\n") + "\n")
	}
	b.WriteString("\nUser question: " + req.Query + "\n\n")
	b.WriteString("Answer the question in Markdown, strictly on the research focus. " +
		"Use the documents as the primary source and web findings only as support. Ignore anything off-focus. " +
		"If the gathered information does not actually answer the question, say so plainly. Cite source URLs.")

	messages := []map[string]string{
		{"role": "system", "content": "You are a thorough research assistant. Respond in Markdown."},
		{"role": "user", "content": b.String()},
	}
	return a.stream(ctx, messages, emit)
}

// assemble the document

func joinDocs(docs []string) string {
	if len(docs) == 0 {
		return "(none provided)\n"
	}
	var b strings.Builder
	for i, d := range docs {
		fmt.Fprintf(&b, "--- Document %d ---\n%s\n\n", i+1, d)
	}
	return b.String()
}

func (a *Agent) complete(ctx context.Context, messages []map[string]string, jsonMode bool) (string, error) {
	body := map[string]any{"model": a.model, "messages": messages}
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	resp, err := a.post(ctx, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm returned %d: %s", resp.StatusCode, msg)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (a *Agent) stream(ctx context.Context, messages []map[string]string, emit func(string) error) error {
	body := map[string]any{"model": a.model, "messages": messages, "stream": true}
	resp, err := a.post(ctx, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm returned %d: %s", resp.StatusCode, msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
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

func (a *Agent) post(ctx context.Context, body map[string]any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return a.http.Do(req)
}
