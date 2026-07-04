package rest

import (
	"encoding/json"
	"net/http"

	"bowatt-backend/agent"
	"bowatt-backend/documents"
)

func (s *Server) handleResearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Request string `json:"request"`
		Source  string `json:"source"` // "auto"(default) | "docs" | "web"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.Request == "" {
		http.Error(w, "request must not be empty", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// gather the pre-embedded chunks cached in the store at upload time
	var chunks []documents.Chunk
	for _, d := range s.Store.All() {
		chunks = append(chunks, d.Chunks...)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	req := agent.Request{
		Query:  body.Request,
		Chunks: chunks,
		Mode:   agent.ParseMode(body.Source)}

	err := s.Agent.Answer(r.Context(), req, func(chunk string) error {
		if _, err := w.Write([]byte(chunk)); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		// add erro code into streaming
		w.Write([]byte("\n\n[error: " + err.Error() + "]"))
		flusher.Flush()
	}
}
