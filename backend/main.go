package main

import (
	"log"
	"net/http"
	"os"

	"bowatt-backend/agent"
	"bowatt-backend/rest"
	"bowatt-backend/store"
)

func main() {
	ag := agent.New(
		os.Getenv("LLM_API_KEY"),
		getenv("LLM_BASE_URL", "https://api.openai.com/v1"),
		getenv("LLM_MODEL", "gpt-4o-mini"),
		getenv("EMBED_MODEL", "text-embedding-3-small"),
		os.Getenv("SEARCH_API_KEY"), // Initialize agent and load the env variables
	)

	// initialize a rest server and inject memory for uploaded sources and research agent
	srv := &rest.Server{
		Store: store.NewMemory(),
		Agent: ag,
	}

	addr := ":8787"
	log.Printf("backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.Routes()))
}

// setup default env variable for model name and base url
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
