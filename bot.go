package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func SendWhatsAppMessage(phone string, message string) error {
	token := os.Getenv("WHATSAPP_ACCESS_TOKEN")
	phoneNumberID := os.Getenv("WHATSAPP_PHONE_ID") // ID del número de origen desde Meta

	if token == "" || phoneNumberID == "" {
		log.Println("WARNING: WHATSAPP_ACCESS_TOKEN o WHATSAPP_PHONE_ID no están configurados. No se enviará mensaje real.")
		return fmt.Errorf("credenciales de whatsapp faltantes")
	}

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phone,
		"type":              "text",
		"text": map[string]interface{}{
			"body": message,
		},
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR sending WhatsApp msg, status: %d", resp.StatusCode)
		return fmt.Errorf("error enviando mensaje whatsapp")
	}
	
	log.Printf("SUCCESS: WhatsApp message sent to %s", phone)
	return nil
}

func ProcessMessage(phone string, text string) {
	log.Printf("Bot received message from %s: %s", phone, text)
	// Example processing: we insert a fake order just to trigger the monitor logic.
	
	// Create order and trigger SSE event
	if DB != nil {
		id, err := InsertOrder(context.Background(), "Cliente WhatsApp", phone, text, "PICKUP", "EFECTIVO", 150.00)
		if err == nil {
			orderData := map[string]interface{}{
				"id":                id,
				"nombre":            "Cliente WhatsApp",
				"telefono":          phone,
				"detalles_orden":    text,
				"direccion_entrega": "PICKUP",
				"metodo_pago":       "EFECTIVO",
				"total":             150.00,
			}
			EmitOrder(orderData) // trigger SSE to kitchen
			
			// Try sending a real reply
			responseMsg := fmt.Sprintf("¡Hola! Hemos recibido tu pedido: '%s'. Se despachará a cocina de inmediato.", text)
			SendWhatsAppMessage(phone, responseMsg)
		} else {
			log.Printf("ERROR inserting order: %v", err)
		}
	} else {
		// Mock order for testing
		orderData := map[string]interface{}{
			"id":                999,
			"nombre":            "Cliente WhatsApp",
			"telefono":          phone,
			"detalles_orden":    text,
			"direccion_entrega": "PICKUP",
			"metodo_pago":       "EFECTIVO",
			"total":             150.00,
		}
		EmitOrder(orderData)
		SendWhatsAppMessage(phone, "¡Hola! Esto es una prueba local (sin base de datos). Recibimos tu mensaje.")
	}
}
