package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"bowatt-backend/documents"
)

// sourceMeta is the metadata view of a stored document (no content). It is the
// shape the file-manager list renders.
type sourceMeta struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Type       string    `json:"type"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// sourceDetail is a single source including its editable text content.
type sourceDetail struct {
	sourceMeta
	Content string `json:"content"`
}

func toMeta(d documents.Document) sourceMeta {
	return sourceMeta{
		ID:         d.ID,
		Name:       d.Name,
		Size:       d.Size,
		Type:       d.Type,
		UploadedAt: d.UploadedAt,
	}
}

type listResult struct {
	Sources []sourceMeta `json:"sources"`
}

type uploadResult struct {
	Uploaded []sourceMeta `json:"uploaded"`
}

// handleSources serves the /api/sources collection:
//
//	GET  -> list all sources (metadata only)
//	POST -> upload one or more files, appending them to the store
func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSources(w, r)
	case http.MethodPost:
		s.uploadSources(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listSources(w http.ResponseWriter, _ *http.Request) {
	docs := s.Store.All()
	out := make([]sourceMeta, 0, len(docs))
	for _, d := range docs {
		out = append(out, toMeta(d))
	}
	writeJSON(w, http.StatusOK, listResult{Sources: out})
}

func (s *Server) uploadSources(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "could not parse upload", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]

	// Append on upload so the file manager accumulates a library of sources;
	// removing a source is now an explicit DELETE rather than a side effect.
	uploaded := make([]sourceMeta, 0, len(files))

	// document processing: extract content, chunk, embed, and store
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			http.Error(w, "could not read file", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			http.Error(w, "could not read file", http.StatusBadRequest)
			return
		}

		doc := documents.Extract(fh.Filename, data)
		chunks, err := s.Agent.IndexDocument(r.Context(), doc.Content)
		if err != nil {
			http.Error(w, "could not embed file: "+err.Error(), http.StatusBadGateway)
			return
		}
		doc.Chunks = chunks
		doc.Type = fh.Header.Get("Content-Type")
		doc.Size = int64(len(doc.Content))
		doc.UploadedAt = time.Now().UTC()

		stored := s.Store.Add(doc)
		uploaded = append(uploaded, toMeta(stored))
	}

	writeJSON(w, http.StatusOK, uploadResult{Uploaded: uploaded})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
