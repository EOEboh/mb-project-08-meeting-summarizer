package main

import (
	"log"
	"net/http"
	"os"

	"github.com/EOEboh/mb-project-08-meeting-summarizer/handlers"
)

func main() {
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Routes
	//   GET  /           => audio upload page
	//   POST /summarize  => transcribe audio + run AI summary, return HTML fragment
	//   POST /export     => download full markdown summary as a file
	mux.HandleFunc("/", handlers.Index)
	mux.HandleFunc("POST /summarize", handlers.Summarize)
	mux.HandleFunc("POST /export", handlers.Export)

	addr := ":" + envOr("PORT", "8080")
	log.Printf("Meeting Summarizer running: http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
