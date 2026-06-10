package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type GeminiDecision struct {
	ResponseText      string   `json:"response_text"`
	SendMenuImage     bool     `json:"send_menu_image"`
	IsOrderComplete   bool     `json:"is_order_complete"`
	OrderDetails      string   `json:"order_details"`
	DeliveryAddress   string   `json:"delivery_address"`
	PaymentMethod     string   `json:"payment_method"`
	Total             float64  `json:"total"`
	InventoryToRemove []string `json:"inventory_to_remove"`
}

type GeminiRequest struct {
	Contents         []MessageContent `json:"contents"`
	SystemInstruction *SystemInst     `json:"system_instruction,omitempty"`
	GenerationConfig  GenConfig       `json:"generation_config"`
}

type MessageContent struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type SystemInst struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenConfig struct {
	ResponseMimeType string `json:"response_mime_type"`
	Temperature      float64 `json:"temperature"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

var botSystemPrompt = `Eres el asistente virtual simpático y experto de SUSHI LOSPLEBES. 
Tu objetivo es ayudar a los clientes a armar su orden de sushi paso a paso por WhatsApp.

Reglas de negocio:
- Un rollo estándar cuesta $120 MXN, un rollo especial $150 MXN (inventa o calcula precios atractivos pero rentables).
- Si el cliente requiere envío a domicilio, suma SIEMPRE $40 MXN al total, aplica para cualquier lugar.
- Si el cliente te saluda por primera vez (el historial está vacío o solo tiene un mensaje), DEBES darle la bienvenida al restaurante SUSHI LOSPLEBES de manera muy amable e indicar "send_menu_image" = true en tu JSON para que el sistema le envíe la foto del menú.
- Métodos de pago aceptados: "Efectivo" o "Transferencia".
- Si el cliente elige "Transferencia", pásale la CLABE: 012345678912345678 a nombre de SUSHI LOSPLEBES y dile que envíe la foto del comprobante por aquí, su orden será validada por un humano. El "payment_method" en el JSON debe ser "TRANSFERENCIA (Por validar comprobante)".
- Para lograr "is_order_complete" = true, debes haber recolectado: qué quieren comer, si es para recoger (PICKUP) o enviar a domicilio (con dirección), y el método de pago (Efectivo o Transferencia).
- Mientras "is_order_complete" sea false, en "response_text" hazles las preguntas necesarias (ej. "¿Gusta envío a domicilio o pasaría por él? ¿Y cómo desea pagar, Efectivo o Transferencia?").

Descuento de Inventario:
Cuando el pedido esté completado ("is_order_complete" = true), calcula los insumos que se consumirán por cada rollo pedido y ponlos en la lista "inventory_to_remove". Por cada 1 rollo de sushi debes descontar aproximadamente:
- "arroz 265g" (calculado del rango 240g-290g)
- "proteinas 50g" 
- "pollo 40g" (sólo si pide de pollo)
- "pepino 20g"
- "zanahoria 15g"
- "cebolla 10g"
- "queso_philadelphia 30g"
- "aderezo 10g"
- "salsa_soya 1"
- "salsa_roja 1"
- "contenedor_7x7 1"
- "p200 1" (envases pequeños para el aderezo y salsa de soya)
- "palillos_chinos 1"
- "aluminio 1"
- "servilletas 2"

ESTRUCTURA STRICTA MULTI-PROPOSITO (SIEMPRE RETORNA JSON en responseMimeType="application/json"):
{
  "response_text": "Texto a enviar por WhatsApp.",
  "send_menu_image": boolean,
  "is_order_complete": boolean,
  "order_details": "Ej: 1x Rollo Empanizado, 1x Té helado. Nota: sin cebollín.",
  "delivery_address": "Calle Falsa 123 o 'PICKUP'",
  "payment_method": "Efectivo",
  "total": 160.0,
  "inventory_to_remove": ["arroz 265g", "queso_philadelphia 30g", "contenedor_7x7 1"]
}
`

var chatMemory = make(map[string]string)

func callGeminiWithModel(model string, apiKey string, requestBody GeminiRequest) ([]byte, int, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	return bodyBytes, resp.StatusCode, err
}

func CallGemini(phone string, userMessage string) (GeminiDecision, error) {
	apiKey := strings.TrimSpace(strings.Trim(os.Getenv("GEMINI_API_KEY"), "\""))
	if apiKey == "" {
		return GeminiDecision{}, fmt.Errorf("GEMINI_API_KEY no configurado")
	}

	// Agregar a historial muy básico (limitar a últimos 500 chars para no crecer infinito)
	historial := chatMemory[phone]
	historial += "\nCliente: " + userMessage
	if len(historial) > 1000 {
		historial = historial[len(historial)-1000:]
	}
	chatMemory[phone] = historial

	reqData := GeminiRequest{
		SystemInstruction: &SystemInst{
			Parts: []Part{{Text: botSystemPrompt}},
		},
		Contents: []MessageContent{
			{
				Role:  "user",
				Parts: []Part{{Text: "Historial de conversación:\n" + historial + "\n\nResponde como el bot analizando el último mensaje y generando el JSON."}},
			},
		},
		GenerationConfig: GenConfig{
			ResponseMimeType: "application/json",
			Temperature:      0.2, // Mantenerlo un poco determinista
		},
	}

	// Fallback strategy to handle latest models in 2026
	modelsToTry := []string{"gemini-3.5-flash", "gemini-flash-latest", "gemini-2.5-flash"}
	var bodyBytes []byte
	var statusCode int
	var err error

	for _, model := range modelsToTry {
		bodyBytes, statusCode, err = callGeminiWithModel(model, apiKey, reqData)
		if err != nil {
			return GeminiDecision{}, err
		}
		if statusCode == http.StatusOK {
			break
		}
		log.Printf("WARNING Gemini returned %d for model %s: %s", statusCode, model, string(bodyBytes))
		if statusCode != http.StatusNotFound && statusCode != http.StatusBadRequest {
			// Si no es un problema del modelo sino de autenticación o quota, salir.
			break
		}
	}

	if statusCode != http.StatusOK {
		LogSystemError("GEMINI_API", "Error completando petición a Gemini", string(bodyBytes), statusCode)
		return GeminiDecision{}, fmt.Errorf("todos los intentos a gemini fallaron. ultimo status code %d: %s", statusCode, string(bodyBytes))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return GeminiDecision{}, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return GeminiDecision{}, fmt.Errorf("gemini devolvió respuesta vacía")
	}

	rawJsonText := geminiResp.Candidates[0].Content.Parts[0].Text
	
	// Strip possible markdown json formatting
	rawJsonText = strings.TrimSpace(rawJsonText)
	if strings.HasPrefix(rawJsonText, "```json") {
		rawJsonText = strings.TrimPrefix(rawJsonText, "```json")
	} else if strings.HasPrefix(rawJsonText, "```") {
		rawJsonText = strings.TrimPrefix(rawJsonText, "```")
	}
	if strings.HasSuffix(rawJsonText, "```") {
		rawJsonText = strings.TrimSuffix(rawJsonText, "```")
	}
	rawJsonText = strings.TrimSpace(rawJsonText)

	var decision GeminiDecision
	if err := json.Unmarshal([]byte(rawJsonText), &decision); err != nil {
		return GeminiDecision{}, fmt.Errorf("error al mapear gemini a decision json: %v", err)
	}

	// Almacenar respuesta del bot en el historial para contexto
	chatMemory[phone] += "\nBot: " + decision.ResponseText

	if decision.IsOrderComplete {
		// Limpiar el historial una vez completada la orden para futuras ordens
		delete(chatMemory, phone)
	}

	return decision, nil
}
