package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		verifyToken := os.Getenv("WHATSAPP_VERIFY_TOKEN")
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && token == verifyToken {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(challenge))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method == "POST" {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("ERROR: decoding webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// In a real app we'd parse the structure and pass the message to ProcessMessage
		ProcessMessage("12345", "Hola")

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
