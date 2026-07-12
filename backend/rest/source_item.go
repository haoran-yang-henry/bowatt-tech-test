package rest

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSourceItem serves /api/sources/{id}:
//
//	GET    -> the source with its editable content
//	PUT    -> save edited content (re-chunk + re-embed)
//	DELETE -> remove the source
func (s *Server) handleSourceItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sources/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "invalid source id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getSource(w, id)
	case http.MethodPut:
		s.updateSource(w, r, id)
	case http.MethodDelete:
		s.deleteSource(w, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getSource(w http.ResponseWriter, id string) {
	doc, ok := s.Store.Get(id)
	if !ok {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sourceDetail{sourceMeta: toMeta(doc), Content: doc.Content})
}

func (s *Server) updateSource(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := s.Store.Get(id); !ok {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Re-index the edited content so retrieval reflects the new text.
	chunks, err := s.Agent.IndexDocument(r.Context(), body.Content)
	if err != nil {
		http.Error(w, "could not embed file: "+err.Error(), http.StatusBadGateway)
		return
	}

	updated, ok := s.Store.Update(id, body.Content, chunks)
	if !ok {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sourceDetail{sourceMeta: toMeta(updated), Content: updated.Content})
}

func (s *Server) deleteSource(w http.ResponseWriter, id string) {
	if !s.Store.Delete(id) {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
