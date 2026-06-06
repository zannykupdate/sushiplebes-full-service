package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// HandleMonitorInterface parsea y sirve la página web estática
func HandleMonitorInterface(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/monitor.html")
	if err != nil {
		log.Printf("ERROR: Fallo al cargar el template monitor.html: %v\n", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Printf("ERROR: Fallo al renderizar el template: %v\n", err)
	}
}

// HandleMonitorStream inicializa el Server-Sent Events (SSE) y sondea base de datos
func HandleMonitorStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming no soportado en este entorno", http.StatusInternalServerError)
		return
	}

	var maxID int
	if DB != nil {
		ctx := context.Background()
		// Obtener el ID más alto actual para no repetir pedidos al recargar
		DB.QueryRow(ctx, "SELECT COALESCE(MAX(id), 0) FROM pedidos").Scan(&maxID)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			log.Println("INFO: Cliente desconectado del monitor SSE.")
			return
		case <-ticker.C:
			if DB == nil {
				continue
			}

			// Buscamos órdenes marcadas como cerradas/completadas por el bot
			rows, err := DB.Query(context.Background(), `
				SELECT id, telefono, nombre, detalles_orden, COALESCE(direccion_entrega, ''), COALESCE(metodo_pago, ''), total
				FROM pedidos
				WHERE estado = 4 AND id > $1
				ORDER BY id ASC
			`, maxID)

			if err != nil && err != pgx.ErrNoRows {
				log.Printf("ERROR: Fallo sondeando la tabla pedidos en el SSE: %v\n", err)
				continue
			}

			newMaxID := maxID
			for rows.Next() {
				type PedidoPayload struct {
					ID               int     `json:"id"`
					Telefono         string  `json:"telefono"`
					Nombre           string  `json:"nombre"`
					DetallesOrden    string  `json:"detalles_orden"`
					DireccionEntrega string  `json:"direccion_entrega"`
					MetodoPago       string  `json:"metodo_pago"`
					Total            float64 `json:"total"`
				}

				var payload PedidoPayload
				err := rows.Scan(&payload.ID, &payload.Telefono, &payload.Nombre, &payload.DetallesOrden, &payload.DireccionEntrega, &payload.MetodoPago, &payload.Total)
				if err != nil {
					log.Printf("ERROR: Falla escaneando row de pedido a estructura: %v\n", err)
					continue
				}

				newMaxID = payload.ID

				bytesJSON, _ := json.Marshal(payload)
				fmt.Fprintf(w, "data: %s\n\n", string(bytesJSON))
				flusher.Flush()
				
				log.Printf("SUCCESS: Enviada señal SSE para imprimir Ticket #%d", payload.ID)
			}
			rows.Close()
			maxID = newMaxID
		}
	}
}
