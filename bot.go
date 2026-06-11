package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var hermosilloLock *time.Location

func init() {
	loc, err := time.LoadLocation("America/Hermosillo")
	if err != nil {
		loc = time.FixedZone("UTC-7", -7*60*60) // Fallback si tzdata no está disponible
	}
	hermosilloLock = loc
}

func SendWhatsAppMessage(phone string, message string) error {
	token := AppConfig.WhatsAppToken
	phoneNumberID := AppConfig.WhatsAppPhoneID // ID del número de origen desde Meta

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

	resp, err := globalHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		LogSystemError("WHATSAPP_API", "Error enviando mensaje", string(bodyBytes), resp.StatusCode)
		SetWhatsAppStatus(resp.StatusCode)
		return fmt.Errorf("error enviando mensaje whatsapp")
	}
	SetWhatsAppStatus(200)
	
	log.Printf("SUCCESS: WhatsApp message sent to %s", phone)
	return nil
}

func SendWhatsAppImage(phone string, imageUrl string) error {
	token := AppConfig.WhatsAppToken
	phoneNumberID := AppConfig.WhatsAppPhoneID

	if token == "" || phoneNumberID == "" || imageUrl == "" {
		return fmt.Errorf("credenciales faltantes o url vacia")
	}

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phone,
		"type":              "image",
		"image": map[string]interface{}{
			"link": imageUrl,
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

	resp, err := globalHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		LogSystemError("WHATSAPP_API", "Error enviando imagen", string(bodyBytes), resp.StatusCode)
		SetWhatsAppStatus(resp.StatusCode)
		return fmt.Errorf("error enviando imagen whatsapp")
	}
	SetWhatsAppStatus(200)
	return nil
}

func ProcessMessage(phone string, text string) {
	log.Printf("Bot received message from %s: %s", phone, text)

	// Horario de atención: 11:00 AM a 11:00 PM (Hora Hermosillo / UTC-7)
	now := time.Now().In(hermosilloLock)
	if now.Hour() < 11 || now.Hour() >= 23 {
		log.Printf("Message received outside business hours from %s", phone)
		SendWhatsAppMessage(phone, "¡Hola! Gracias por comunicarte con Sushi Los Plebes. 🍣\n\nActualmente nuestro restaurante se encuentra cerrado. Nuestro horario de atención es todos los días de 11:00 AM a 11:00 PM.\n\nPor favor contáctanos mañana en horario laboral y con gusto tomaremos tu pedido. ¡Que pases buena noche! 🌙")
		return // Do not process state, stop right here
	}
	
	decision, err := CallGemini(phone, text)
	if err != nil {
		log.Printf("ERROR from Gemini: %v", err)
		SendWhatsAppMessage(phone, "Estamos un poco saturados recibiendo pedidos en este momento. 🍣 Por favor, vuelve a escribirnos en 5 minutos. ¡Agradecemos tu paciencia! 🙏")
		return
	}

	// Enviamos el mensaje de texto primero
	SendWhatsAppMessage(phone, decision.ResponseText)

	// Enviamos imagen del menú si Gemini lo decide
	if decision.SendMenuImage {
		menuUrl := AppConfig.MenuImageURL
		if menuUrl == "" {
			// Placeholder si no está configurado
			menuUrl = "https://i.imgur.com/3q17vT9.jpeg" // Imagen ejemplo de sushi
		}
		SendWhatsAppImage(phone, menuUrl)
	}

	if decision.RequiresHuman {
		if DB != nil {
			DB.Exec(context.Background(), "INSERT INTO support_tickets (telefono, mensaje) VALUES ($1, $2)", phone, text)
		}
	} else if decision.IsOrderComplete {
		customerName := decision.CustomerName
		if customerName == "" {
			customerName = "Cliente " + phone // Fallback
		}

		// La orden está lista para meter a Base de datos y Monitor
		if DB != nil {
			id, err := InsertOrder(context.Background(), customerName, phone, decision.OrderDetails, decision.DeliveryAddress, decision.PaymentMethod, decision.Subtotal, decision.Tax, decision.Shipping, decision.Total, decision.InventoryToRemove)
			if err == nil {
				orderData := map[string]interface{}{
					"id":                id,
					"nombre":            customerName,
					"telefono":          phone,
					"detalles_orden":    decision.OrderDetails,
					"direccion_entrega": decision.DeliveryAddress,
					"metodo_pago":       decision.PaymentMethod,
					"subtotal":          decision.Subtotal,
					"tax":               decision.Tax,
					"shipping":          decision.Shipping,
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
				"nombre":            customerName,
				"telefono":          phone,
				"detalles_orden":    decision.OrderDetails,
				"direccion_entrega": decision.DeliveryAddress,
				"metodo_pago":       decision.PaymentMethod,
				"subtotal":          decision.Subtotal,
				"tax":               decision.Tax,
				"shipping":          decision.Shipping,
				"total":             decision.Total,
			}
			EmitOrder(orderData)
		}
	}
}

