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
	
	decision, err := CallGemini(phone, text)
	if err != nil {
		log.Printf("ERROR from Gemini: %v", err)
		SendWhatsAppMessage(phone, "¡Ups! Ocurrió un error procesando tu mensaje. Intenta de nuevo.")
		return
	}

	// Responder al usuario lo que Gemini decidió decirles
	SendWhatsAppMessage(phone, decision.ResponseText)

	if decision.IsOrderComplete {
		// La orden está lista para meter a Base de datos y Monitor
		if DB != nil {
			id, err := InsertOrder(context.Background(), phone, decision.OrderDetails, decision.DeliveryAddress, decision.PaymentMethod, decision.Total, decision.InventoryToRemove)
			if err == nil {
				orderData := map[string]interface{}{
					"id":                id,
					"nombre":            "Cliente " + phone,
					"telefono":          phone,
					"detalles_orden":    decision.OrderDetails,
					"direccion_entrega": decision.DeliveryAddress,
					"metodo_pago":       decision.PaymentMethod,
					"total":             decision.Total,
				}
				EmitOrder(orderData) // trigger SSE to kitchen
			} else {
				log.Printf("ERROR inserting complete order: %v", err)
			}
		} else {
			// Fallback local mock
			orderData := map[string]interface{}{
				"id":                999,
				"nombre":            "Cliente Local",
				"telefono":          phone,
				"detalles_orden":    decision.OrderDetails,
				"direccion_entrega": decision.DeliveryAddress,
				"metodo_pago":       decision.PaymentMethod,
				"total":             decision.Total,
			}
			EmitOrder(orderData)
		}
	}
}

