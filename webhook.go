package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB es el pool de conexiones global a PostgreSQL (se asume inicializado en db.go)
var DB *pgxpool.Pool

// WhatsAppWebhookPayload representa la estructura JSON del webhook de Meta (WhatsApp)
type WhatsAppWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text"`
					Type string `json:"type"`
				} `json:"messages"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

// WebhookHandler maneja las peticiones GET (verificación) y POST (mensajes) del webhook
func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGetWebhook(w, r)
	case http.MethodPost:
		handlePostWebhook(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetWebhook procesa la verificación del webhook requerida por Meta
func handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	verifyToken := os.Getenv("WHATSAPP_VERIFY_TOKEN")
	if verifyToken == "" {
		log.Println("ERROR: WHATSAPP_VERIFY_TOKEN no está configurado en las variables de entorno")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == verifyToken {
		log.Println("SUCCESS: Webhook verificado correctamente por Meta")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}

	log.Printf("ERROR: Falló la verificación del webhook. mode=%s, token_recibido=%s", mode, token)
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// handlePostWebhook procesa y almacena los mensajes entrantes de WhatsApp
func handlePostWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: No se pudo leer el cuerpo del webhook: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload WhatsAppWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("ERROR: Falló al parsear el struct JSON del webhook: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// REQUERIMIENTO: Responder 200 OK a Meta de inmediato para evitar bloqueos y reintentos.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("EVENT_RECEIVED"))

	if len(payload.Entry) == 0 {
		return
	}

	for _, entry := range payload.Entry {
		if len(entry.Changes) == 0 {
			continue
		}

		changeValue := entry.Changes[0].Value

		// Ignoramos si no hay mensajes en este change (ej. eventos de lectura o entrega)
		if len(changeValue.Messages) == 0 {
			continue
		}

		message := changeValue.Messages[0]

		// Solamente procesamos mensajes de texto por el momento
		if message.Type != "text" {
			log.Printf("INFO: Mensaje omitido por no ser de texto. Tipo recibido: %s", message.Type)
			continue
		}

		fromNumber := message.From
		textBody := message.Text.Body
		contactName := "Desconocido"

		if len(changeValue.Contacts) > 0 {
			contactName = changeValue.Contacts[0].Profile.Name
		}

		log.Printf("INFO: Mensaje de texto procesado - De: %s (%s) | Texto: %s", fromNumber, contactName, textBody)

		// Volver a formatear el cuerpo original completo a JSON explícito si fuese necesario, 
		// pero utilizaremos el body exacto parseado inicialmente para conservar la integridad.
		
		if DB != nil {
			query := `
				INSERT INTO mensajes_raw (telefono, payload, creado_en)
				VALUES ($1, $2, NOW())
			`
			// Inserción síncrona según requerimientos
			_, err = DB.Exec(context.Background(), query, fromNumber, body)
			if err != nil {
				log.Printf("ERROR: Falla de inserción en Base de Datos para el número %s: %v", fromNumber, err)
			} else {
				log.Printf("SUCCESS: Mensaje original de %s guardado en tabla 'mensajes_raw'", fromNumber)
				// Invocación síncrona al motor de reglas del bot, directo y eficiente
				ProcesarMensaje(fromNumber, contactName, textBody)
			}
		} else {
			log.Println("ERROR CRITICO: Variable DB no está inicializada. No se persisten datos de WhatsApp.")
		}
	}
}
