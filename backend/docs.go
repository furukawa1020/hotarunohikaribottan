package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// docsRoot is the directory from which Markdown documentation files are served.
// It defaults to the project root (one level above the backend/ directory).
var docsRoot = ".."

// handleMarkdown serves Markdown documentation files from docsRoot.
// Files are referenced by name (with or without the .md extension), for example:
//
//	GET /docs/API          → returns API.md
//	GET /docs/REQUIREMENTS → returns REQUIREMENTS.md
func handleMarkdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL path (strip /docs/ prefix)
	name := strings.TrimPrefix(r.URL.Path, "/docs/")
	name = strings.TrimSpace(name)
	if name == "" {
		http.Error(w, "Bad Request: filename required", http.StatusBadRequest)
		return
	}

	// Allow only a plain filename (no path separators)
	if strings.ContainsAny(name, `/\`) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Ensure .md extension
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}

	// Resolve the docs root to an absolute path
	absRoot, err := filepath.Abs(docsRoot)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build and canonicalize the target path
	mdPath, err := filepath.Abs(filepath.Join(absRoot, name))
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Ensure the resolved path is within the docs root directory
	if !strings.HasPrefix(mdPath, absRoot+string(filepath.Separator)) && mdPath != absRoot {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(content); err != nil {
		log.Printf("handleMarkdown: write error: %v", err)
	}
}
