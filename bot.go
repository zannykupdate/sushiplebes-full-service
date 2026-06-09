package main

import (
	"context"
	"fmt"
	"log"
)

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
	}

	// Ideally here we send a whatsapp response back
	fmt.Printf("Sending Whatsapp msg back to %s\n", phone)
}
