package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		verifyToken := AppConfig.WhatsAppVerifyToken
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
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("ERROR: reading webhook body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		appSecret := AppConfig.WhatsAppAppSecret
		if appSecret != "" {
			signature := r.Header.Get("X-Hub-Signature-256")
			if signature == "" || len(signature) <= 7 || signature[:7] != "sha256=" {
				log.Println("ERROR: Invalid or missing X-Hub-Signature-256")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			
			mac := hmac.New(sha256.New, []byte(appSecret))
			mac.Write(bodyBytes)
			expectedMAC := mac.Sum(nil)
			expectedSignature := "sha256=" + hex.EncodeToString(expectedMAC)

			if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
				log.Println("ERROR: Webhook HMAC signature mismatch")
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
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
							go ProcessMessage(phone, text)
							w.WriteHeader(http.StatusOK)
							return
						}
					}
				}
			}
		}
		
		// Fallback por si la estructura del JSON no es de un mensaje entrante estándar
		log.Println("Webhook event did not contain a valid text message payload (might be a status update). Ignored.")

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
