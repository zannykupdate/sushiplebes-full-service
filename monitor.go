package main

import (
	"encoding/json"
	"log"
	"net/http"
	"html/template"
	"sync"
)

var (
	clients   = make(map[chan []byte]bool)
	clientsMu sync.Mutex
)

// HandleMonitorInterface serves the static HTML page for the kitchen monitor
func HandleMonitorInterface(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/monitor.html")
	if err != nil {
		log.Printf("ERROR: Could not load monitor template: %v", err)
		http.Error(w, "Error loading monitor template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// HandleMonitorStream is the SSE endpoint
func HandleMonitorStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan []byte)

	clientsMu.Lock()
	clients[clientChan] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, clientChan)
		clientsMu.Unlock()
		close(clientChan)
	}()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg := <-clientChan:
			w.Write([]byte("data: "))
			w.Write(msg)
			w.Write([]byte("\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// EmitOrder sends a new order to all connected kitchen monitors via SSE
func EmitOrder(data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("EmitOrder err: %v", err)
		return
	}
	
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for clientChan := range clients {
		clientChan <- b
	}
}
