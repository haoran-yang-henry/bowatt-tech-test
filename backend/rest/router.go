package rest

import (
	"net/http"

	"bowatt-backend/agent"
	"bowatt-backend/store"
)

type Server struct {
	Store *store.Memory
	Agent *agent.Agent
}

// Register API routes and apply CORS middleware.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sources", s.handleSources)
	mux.HandleFunc("/api/research", s.handleResearch)
	return withCORS(mux)
}

// allow frontend 5173 to visit backend 8787
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
