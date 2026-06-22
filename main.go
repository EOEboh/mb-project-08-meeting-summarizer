package main

import (
	"log"
	"net/http"
	"os"

	"github.com/EOEboh/mb-project-08-meeting-summarizer/handlers"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists.
	// godotenv.Load() silently does nothing if .env is missing, so this
	// is safe to call unconditionally. Variables already set in the shell
	// environment take precedence over values in .env.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", handlers.Index)
	mux.HandleFunc("POST /summarize", handlers.Summarize)
	mux.HandleFunc("POST /export", handlers.Export)

	addr := ":" + envOr("PORT", "8080")
	log.Printf("Meeting Summarizer running → http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
