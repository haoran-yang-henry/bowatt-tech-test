package rest

import (
	"encoding/json"
	"io"
	"net/http"

	"bowatt-backend/documents"
)

type uploadedFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

type uploadResult struct {
	Uploaded []uploadedFile `json:"uploaded"`
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "could not parse upload", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	uploaded := make([]uploadedFile, 0, len(files))
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

		s.Store.Save(documents.Extract(fh.Filename, data))
		uploaded = append(uploaded, uploadedFile{
			Name: fh.Filename,
			Size: fh.Size,
			Type: fh.Header.Get("Content-Type"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadResult{Uploaded: uploaded})
}
