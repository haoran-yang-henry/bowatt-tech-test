package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"bowatt-backend/agent"
	"bowatt-backend/store"
)

// mockLLM stands in for the OpenAI-compatible API the agent talks to. It
// serves fixed vectors on /embeddings and canned completions on
// /chat/completions, and records the prompt of every streaming (synthesize)
// call so tests can assert which documents reached the final answer.
type mockLLM struct {
	*httptest.Server
	mu            sync.Mutex
	streamPrompts []string
}

func newMockLLM(t *testing.T) *mockLLM {
	t.Helper()
	m := &mockLLM{}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/embeddings":
			var body struct {
				Input []string `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			type item struct {
				Embedding []float32 `json:"embedding"`
			}
			data := make([]item, len(body.Input))
			for i := range data {
				data[i] = item{Embedding: []float32{0.1, 0.2, 0.3}}
			}
			json.NewEncoder(w).Encode(map[string]any{"data": data})

		case "/chat/completions":
			var body struct {
				Stream         bool            `json:"stream"`
				ResponseFormat json.RawMessage `json:"response_format"`
				Messages       []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			switch {
			case body.Stream:
				// The synthesize step. Record its prompt for assertions.
				m.mu.Lock()
				m.streamPrompts = append(m.streamPrompts, body.Messages[len(body.Messages)-1].Content)
				m.mu.Unlock()
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, `data: {"choices":[{"delta":{"content":"mock "}}]}`+"\n\n")
				io.WriteString(w, `data: {"choices":[{"delta":{"content":"answer"}}]}`+"\n\n")
				io.WriteString(w, "data: [DONE]\n\n")
			case body.ResponseFormat != nil:
				// Evaluator/planner JSON calls: report full coverage so the
				// research loop converges after one round.
				json.NewEncoder(w).Encode(chatResponse(`{"covered": true, "gaps": []}`))
			default:
				// The derive-focus call.
				json.NewEncoder(w).Encode(chatResponse("the mock research focus"))
			}

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(m.Close)
	return m
}

func chatResponse(content string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	}
}

func (m *mockLLM) lastStreamPrompt(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.streamPrompts) == 0 {
		t.Fatal("no streaming LLM call was made")
	}
	return m.streamPrompts[len(m.streamPrompts)-1]
}

// newTestAPI wires the real router and a fresh store to the mock LLM and
// serves them over real HTTP, so every test drives the exact code path
// production traffic takes (routing, CORS middleware, JSON, multipart).
func newTestAPI(t *testing.T) (*httptest.Server, *mockLLM) {
	t.Helper()
	llm := newMockLLM(t)
	srv := &Server{
		Store: store.NewMemory(),
		Agent: agent.New("test-key", llm.URL, "test-model", "test-embed", ""),
	}
	api := httptest.NewServer(srv.Routes())
	t.Cleanup(api.Close)
	return api, llm
}

type testFile struct{ name, content string }

func uploadFiles(t *testing.T, api *httptest.Server, files ...testFile) uploadResult {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		part, err := mw.CreateFormFile("files", f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte(f.content)); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	mw.Close()

	resp, err := http.Post(api.URL+"/api/sources", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload returned %d: %s", resp.StatusCode, msg)
	}
	var out uploadResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	return out
}

func doJSON(t *testing.T, method, url string, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func listAll(t *testing.T, api *httptest.Server) []sourceMeta {
	t.Helper()
	resp, err := http.Get(api.URL + "/api/sources")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	var out listResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return out.Sources
}

func TestUploadAppendsToList(t *testing.T) {
	api, _ := newTestAPI(t)

	first := uploadFiles(t, api, testFile{"a.txt", "alpha"})
	if len(first.Uploaded) != 1 || first.Uploaded[0].Name != "a.txt" || first.Uploaded[0].Size != 5 {
		t.Fatalf("unexpected upload result: %+v", first.Uploaded)
	}

	// A second upload must append, not replace.
	uploadFiles(t, api, testFile{"b.txt", "beta"}, testFile{"c.txt", "gamma"})

	sources := listAll(t, api)
	if len(sources) != 3 {
		t.Fatalf("expected 3 sources after two uploads, got %d: %+v", len(sources), sources)
	}
}

func TestGetSourceReturnsContent(t *testing.T) {
	api, _ := newTestAPI(t)
	up := uploadFiles(t, api, testFile{"a.txt", "the file body"})

	resp := doJSON(t, http.MethodGet, api.URL+"/api/sources/"+up.Uploaded[0].ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get returned %d", resp.StatusCode)
	}
	var detail sourceDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Content != "the file body" || detail.Name != "a.txt" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestUpdateSourceRewritesContent(t *testing.T) {
	api, _ := newTestAPI(t)
	up := uploadFiles(t, api, testFile{"a.txt", "old"})
	id := up.Uploaded[0].ID

	resp := doJSON(t, http.MethodPut, api.URL+"/api/sources/"+id, `{"content":"brand new text"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put returned %d", resp.StatusCode)
	}
	var updated sourceDetail
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if updated.Content != "brand new text" || updated.Size != int64(len("brand new text")) {
		t.Fatalf("content/size not updated: %+v", updated)
	}
	if updated.ID != id || updated.Name != "a.txt" {
		t.Fatalf("identity changed on update: %+v", updated)
	}

	// The edit must be visible on a subsequent read.
	get := doJSON(t, http.MethodGet, api.URL+"/api/sources/"+id, "")
	defer get.Body.Close()
	var detail sourceDetail
	if err := json.NewDecoder(get.Body).Decode(&detail); err != nil {
		t.Fatalf("decode get after put: %v", err)
	}
	if detail.Content != "brand new text" {
		t.Fatalf("edit did not persist: %q", detail.Content)
	}
}

func TestDeleteSourceRemovesIt(t *testing.T) {
	api, _ := newTestAPI(t)
	up := uploadFiles(t, api, testFile{"a.txt", "alpha"}, testFile{"b.txt", "beta"})
	id := up.Uploaded[0].ID

	resp := doJSON(t, http.MethodDelete, api.URL+"/api/sources/"+id, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete returned %d, want 204", resp.StatusCode)
	}

	get := doJSON(t, http.MethodGet, api.URL+"/api/sources/"+id, "")
	get.Body.Close()
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete returned %d, want 404", get.StatusCode)
	}

	if sources := listAll(t, api); len(sources) != 1 || sources[0].Name != "b.txt" {
		t.Fatalf("wrong sources after delete: %+v", sources)
	}
}

func TestErrorStatusCodes(t *testing.T) {
	api, _ := newTestAPI(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"get unknown source", http.MethodGet, "/api/sources/nope", "", http.StatusNotFound},
		{"put unknown source", http.MethodPut, "/api/sources/nope", `{"content":"x"}`, http.StatusNotFound},
		{"delete unknown source", http.MethodDelete, "/api/sources/nope", "", http.StatusNotFound},
		{"empty source id", http.MethodGet, "/api/sources/", "", http.StatusBadRequest},
		{"patch collection", http.MethodPatch, "/api/sources", "", http.StatusMethodNotAllowed},
		{"post to item", http.MethodPost, "/api/sources/abc", "", http.StatusMethodNotAllowed},
		{"get research", http.MethodGet, "/api/research", "", http.StatusMethodNotAllowed},
		{"research empty request", http.MethodPost, "/api/research", `{"request":""}`, http.StatusBadRequest},
		{"research invalid json", http.MethodPost, "/api/research", `{`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, tc.method, api.URL+tc.path, tc.body)
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("%s %s returned %d, want %d", tc.method, tc.path, resp.StatusCode, tc.want)
			}
		})
	}
}

func TestResearchStreamsAnswer(t *testing.T) {
	api, _ := newTestAPI(t)
	uploadFiles(t, api, testFile{"a.txt", "a document about honeybees"})

	resp := doJSON(t, http.MethodPost, api.URL+"/api/research",
		`{"request":"tell me about the docs","source":"docs"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("research returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	out := string(body)
	if !strings.Contains(out, "the mock research focus") {
		t.Fatalf("stream missing the focus line: %q", out)
	}
	if !strings.Contains(out, "mock answer") {
		t.Fatalf("stream missing the synthesized answer: %q", out)
	}
}

// TestResearchFiltersSelectedSources is the contract behind the file manager's
// checkboxes: source_ids restricts research to the selected documents, and an
// empty selection means all of them. The synthesize prompt captured by the
// mock LLM reveals which documents were actually retrieved.
func TestResearchFiltersSelectedSources(t *testing.T) {
	api, llm := newTestAPI(t)
	up := uploadFiles(t, api,
		testFile{"bees.txt", "a document about honeybees"},
		testFile{"volcano.txt", "a document about volcanoes"},
	)
	beesID := up.Uploaded[0].ID

	// Selecting only bees.txt must keep volcanoes out of the answer context.
	resp := doJSON(t, http.MethodPost, api.URL+"/api/research",
		`{"request":"what do the docs say?","source":"docs","source_ids":["`+beesID+`"]}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	prompt := llm.lastStreamPrompt(t)
	if !strings.Contains(prompt, "honeybees") {
		t.Fatalf("selected document missing from synthesize prompt: %q", prompt)
	}
	if strings.Contains(prompt, "volcanoes") {
		t.Fatalf("unselected document leaked into synthesize prompt: %q", prompt)
	}

	// No selection falls back to researching every uploaded source.
	resp = doJSON(t, http.MethodPost, api.URL+"/api/research",
		`{"request":"what do the docs say?","source":"docs"}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	prompt = llm.lastStreamPrompt(t)
	if !strings.Contains(prompt, "honeybees") || !strings.Contains(prompt, "volcanoes") {
		t.Fatalf("empty selection should include all documents, prompt: %q", prompt)
	}
}
