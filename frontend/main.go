package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
)

//go:embed assets/*
var staticFS embed.FS

func main() {
	port := getenv("PORT", "8083")
	sub, err := fs.Sub(staticFS, "assets")
	if err != nil {
		log.Fatalf("embed fs: %v", err)
	}
	fsHandler := http.FileServer(http.FS(sub))

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// serve index for root or unknown paths
		if r.URL.Path == "/" || path.Ext(r.URL.Path) == "" {
			data, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.Error(w, "index not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		fsHandler.ServeHTTP(w, r)
	}))

	log.Printf("frontend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("frontend server: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

