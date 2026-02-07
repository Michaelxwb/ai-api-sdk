package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/examples/plugin-platform/server"
	"github.com/Michaelxwb/ai-api-sdk/examples/plugin-platform/storage"
)

func main() {
	locators := storage.NewLocatorStore()
	configs := server.NewConfigStore()
	ws := server.NewWebSocketServer(locators)
	api := server.NewAPI(ws, locators, configs)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.HandleWS)
	mux.HandleFunc("/api/configs", api.HandleConfigs)
	mux.HandleFunc("/api/configs/", api.HandleConfigAction)
	mux.HandleFunc("/api/chat", api.HandleChat)

	staticDir := filepath.Join("examples", "plugin-platform", "static")
	if _, err := os.Stat(staticDir); err != nil {
		staticDir = "static"
	}
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("plugin platform listening on http://localhost%s", server.Addr)
	log.Printf("websocket endpoint: ws://localhost%s/ws?configId=YOUR_CONFIG_ID", server.Addr)
	log.Printf("sdk websocket endpoint: ws://localhost%s/ws?role=client&configId=YOUR_CONFIG_ID", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
