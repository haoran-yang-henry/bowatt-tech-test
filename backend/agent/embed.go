package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"

	"bowatt-backend/documents"
)

// chunk splits text into ~chunkSize-char pieces with overlap-char overlap.
func chunk(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= chunkSize {
		return []string{text}
	}
	var chunks []string
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}

// embed calls the OpenAI-compatible /embeddings endpoint for a batch of inputs.
func (a *Agent) embed(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{
		"model": a.embedModel,
		"input": inputs,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embeddings returned %d: %s", resp.StatusCode, msg)
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// IndexDocument splits text into chunks and embeds each one. The resulting
// text+vector pairs are cached in the store at upload time (short-term memory),
// so a document is embedded once regardless of how many questions follow.
func (a *Agent) IndexDocument(ctx context.Context, text string) ([]documents.Chunk, error) {
	pieces := chunk(text, 800, 100)
	if len(pieces) == 0 {
		return nil, nil
	}
	vecs, err := a.embed(ctx, pieces)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(pieces) {
		return nil, fmt.Errorf("embedding count mismatch: got %d want %d", len(vecs), len(pieces))
	}
	out := make([]documents.Chunk, len(pieces))
	for i := range pieces {
		out[i] = documents.Chunk{Text: pieces[i], Vector: vecs[i]}
	}
	return out, nil
}

// cosine returns the cosine similarity between two vectors.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// retrieve embeds the focus and ranks the pre-embedded document chunks by cosine
// similarity, returning the top-k texts. When there are fewer than topK chunks
// they are all returned, so small uploads pass through effectively whole.
func (a *Agent) retrieve(ctx context.Context, focus string, chunks []documents.Chunk, topK int) ([]string, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	vecs, err := a.embed(ctx, []string{focus})
	if err != nil {
		return nil, err
	}
	focusVec := vecs[0]

	type scored struct {
		text  string
		score float64
	}
	ranked := make([]scored, len(chunks))
	for i, c := range chunks {
		ranked[i] = scored{text: c.Text, score: cosine(focusVec, c.Vector)}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if topK > len(ranked) {
		topK = len(ranked)
	}
	out := make([]string, topK)
	for i := 0; i < topK; i++ {
		out[i] = ranked[i].text
	}
	return out, nil
}
