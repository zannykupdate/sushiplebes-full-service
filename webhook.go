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
		
		// Extraer los datos reales del payload de WhatsApp
		if entry, ok := payload["entry"].([]interface{}); ok && len(entry) > 0 {
			if changes, ok := entry[0].(map[string]interface{})["changes"].([]interface{}); ok && len(changes) > 0 {
				if value, ok := changes[0].(map[string]interface{})["value"].(map[string]interface{}); ok {
					if messages, ok := value["messages"].([]interface{}); ok && len(messages) > 0 {
						msg := messages[0].(map[string]interface{})
						
						var phone, text string
						if from, ok := msg["from"].(string); ok {
							phone = from
						}
						
						if msgType, ok := msg["type"].(string); ok && msgType == "text" {
							if textObj, ok := msg["text"].(map[string]interface{}); ok {
								if body, ok := textObj["body"].(string); ok {
									text = body
								}
							}
						}
						
						// Si logramos extraer número y texto, procesamos el mensaje real
						if phone != "" && text != "" {
							ProcessMessage(phone, text)
							w.WriteHeader(http.StatusOK)
							return
						}
					}
				}
			}
		}
		
		// Fallback por si la estructura del JSON no es de un mensaje entrante estándar
		ProcessMessage("Desconocido", "Mensaje sin formato de texto detectado")

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
