package rest

import (
	"encoding/json"
	"net/http"

	"bowatt-backend/agent"
)

func (s *Server) handleResearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Request string `json:"request"`
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

	// retake the documents that has been save into "store"
	var docs []string
	for _, d := range s.Store.All() {
		docs = append(docs, d.Content)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	req := agent.Request{Query: body.Request, Documents: docs}
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
