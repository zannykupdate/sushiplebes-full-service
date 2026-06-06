package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ProcesarMensaje maneja el flujo del embudo de ventas en WhatsApp mediante una máquina de estados
func ProcesarMensaje(telefono, nombre, mensaje string) {
	if DB == nil {
		log.Println("ERROR CRITICO: Variable DB no está inicializada dentro de bot.go. Imposible procesar pedido.")
		return
	}

	ctx := context.Background()

	// 1. Buscar si hay un pedido activo (estado < 4) para simular la persistencia de la conversación
	var pedidoID int
	var estado int
	querySelect := `SELECT id, estado FROM pedidos WHERE telefono = $1 AND estado < 4 ORDER BY id DESC LIMIT 1`

	err := DB.QueryRow(ctx, querySelect, telefono).Scan(&pedidoID, &estado)
	if err != nil {
		if err == pgx.ErrNoRows {
			estado = 0 // Estado 0: Nuevo cliente o nueva orden
		} else {
			log.Printf("ERROR: Fallo al consultar estado activo para el teléfono %s: %v\n", telefono, err)
			return
		}
	}

	log.Printf("INFO: Procesando mensaje entrante de %s en Estado de Motor: %d\n", telefono, estado)

	// 2. Ejecutar la lógica del Árbol de Decisiones
	switch estado {
	case 0:
		// Estado 0 (Nuevo): Crear registro inicial y saludar
		queryInsert := `INSERT INTO pedidos (telefono, nombre, estado) VALUES ($1, $2, 1) RETURNING id`
		err := DB.QueryRow(ctx, queryInsert, telefono, nombre).Scan(&pedidoID)
		if err != nil {
			log.Printf("ERROR: Fallo al insertar nuevo pedido para %s: %v\n", telefono, err)
			return
		}

		respuesta := fmt.Sprintf("¡Hola %s! Bienvenido a SUSHI LOSPLEBES 🍣\n\nAquí tienes nuestro Menú: https://wa.me/c/sushilosplebes\n\nPor favor, escribe tu orden detallada en UN SOLO MENSAJE.", nombre)
		EnviarMensajeWhatsApp(telefono, respuesta)
		log.Printf("SUCCESS: Nombramiento de Estado 0 a 1 exitoso. Pedido ID: %d", pedidoID)

	case 1:
		// Estado 1 (Esperando Orden): Guardar el texto libre
		queryUpdate := `UPDATE pedidos SET detalles_orden = $1, estado = 2 WHERE id = $2`
		_, err := DB.Exec(ctx, queryUpdate, mensaje, pedidoID)
		if err != nil {
			log.Printf("ERROR: Fallo actualizando detalles de la orden en pedido %d: %v\n", pedidoID, err)
			return
		}

		respuesta := "¿Tu pedido será para Recoger en sucursal (escribe PICKUP) o Servicio a Domicilio (escribe DELIVERY)?"
		EnviarMensajeWhatsApp(telefono, respuesta)

	case 2:
		// Estado 2 (Esperando Método/Dirección)
		msgUpper := strings.ToUpper(strings.TrimSpace(mensaje))
		direccion := mensaje
		totalMVP := 150.00 // MVP hardcodeado como se especificó
		
		if msgUpper == "PICKUP" {
			direccion = "PICKUP (Sucursal)"
		}

		queryUpdate := `UPDATE pedidos SET direccion_entrega = $1, total = $2, estado = 3 WHERE id = $3`
		_, err := DB.Exec(ctx, queryUpdate, direccion, totalMVP, pedidoID)
		if err != nil {
			log.Printf("ERROR: Fallo guardando detalles de dirección/método en pedido %d: %v\n", pedidoID, err)
			return
		}

		respuesta := "¿Cómo deseas pagar? Escribe EFECTIVO (indica con cuánto vas a pagar para llevarte cambio) o TRANSFERENCIA."
		EnviarMensajeWhatsApp(telefono, respuesta)

	case 3:
		// Estado 3 (Cierre de Orden y Automatización Síncrona POS/Logística)
		queryUpdate := `UPDATE pedidos SET metodo_pago = $1, estado = 4 WHERE id = $2`
		_, err := DB.Exec(ctx, queryUpdate, mensaje, pedidoID)
		if err != nil {
			log.Printf("ERROR: Fallo cerrando la orden con el pago en pedido %d: %v\n", pedidoID, err)
			return
		}

		// Insertar en tabla ganancias (Impacto estricto POS)
		var total float64
		err = DB.QueryRow(ctx, `SELECT total FROM pedidos WHERE id = $1`, pedidoID).Scan(&total)
		if err == nil {
			queryGanancias := `INSERT INTO ganancias (pedido_id, monto) VALUES ($1, $2)`
			_, errGt := DB.Exec(ctx, queryGanancias, pedidoID, total)
			if errGt != nil {
				log.Printf("ERROR: Falla de base de datos impactando informe de ganancias para pedido %d: %v\n", pedidoID, errGt)
			} else {
				log.Printf("SUCCESS: Ganancia registrada síncronamente para pedido %d por un valor de $%.2f\n", pedidoID, total)
			}
		}

		// Update en inventarios (Simulación del stock dinámico)
		queryStockArroz := `UPDATE inventario SET cantidad = cantidad - 1 WHERE insumo = 'Arroz'`
		queryStockSalmon := `UPDATE inventario SET cantidad = cantidad - 1 WHERE insumo = 'Salmón'`
		
		_, errIn1 := DB.Exec(ctx, queryStockArroz)
		if errIn1 != nil { log.Printf("ERROR: Fallo al reducir stock de Arroz: %v\n", errIn1) }
		
		_, errIn2 := DB.Exec(ctx, queryStockSalmon)
		if errIn2 != nil { log.Printf("ERROR: Fallo al reducir stock de Salmón: %v\n", errIn2) }

		// Salida y finalización hacia el cliente 
		respuesta := "¡Listo! Tu pedido ha sido enviado a la cocina de Los Plebes 🍣🔥"
		EnviarMensajeWhatsApp(telefono, respuesta)
		log.Printf("SUCCESS: Pedido %d completado de lado del Bot WhatsApp. Esperando lectura por SSE.\n", pedidoID)

	default:
		log.Printf("INFO: Anomalía evitada. El pedido en memoria %d está procesando un estado no catalogado (%d).\n", pedidoID, estado)
	}
}

// EnviarMensajeWhatsApp efectúa una llamada HTTP limpia y nativa hacia la API de WhatsApp Cloud 
func EnviarMensajeWhatsApp(telefono, texto string) {
	token := os.Getenv("WHATSAPP_ACCESS_TOKEN")
	phoneID := os.Getenv("WHATSAPP_PHONE_ID")

	if token == "" || phoneID == "" {
		log.Println("ERROR: Variables de entorno necesarias para la API gráfica (WHATSAPP_ACCESS_TOKEN o WHATSAPP_PHONE_ID) no están configuradas.")
		return
	}

	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", phoneID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                telefono,
		"type":              "text",
		"text": map[string]string{
			"body": texto,
		},
	}

	bodyData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Fallo codificando payload string a JSON Byte. Detalle: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyData))
	if err != nil {
		log.Printf("ERROR: Fallo creando objeto HttpRequest de salida. Detalle: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Fallo en red enviando el POST nativo a Meta Graph. Detalle: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("ERROR Meta Graph API rechazado: HTTP Status: %d - Respuesta: %s\n", resp.StatusCode, string(respBody))
		return
	}

	log.Printf("SUCCESS: Mensaje enviado exitosamente vía WhatsApp a -> %s\n", telefono)
}
