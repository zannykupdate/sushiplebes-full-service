package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// enableCors adds CORS headers for frontend accessibility
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func HandleOrdersAPI(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if DB == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		rows, err := DB.Query(context.Background(), "SELECT id, nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, total, status FROM orders ORDER BY id DESC")
		if err != nil {
			log.Printf("ERROR: GET /api/orders failed: %v", err)
			http.Error(w, `{"error": "Failed to fetch orders"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var orders []map[string]interface{}
		for rows.Next() {
			var id int
			var nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, status string
			var total float64
			if err := rows.Scan(&id, &nombre, &telefono, &detalles_orden, &direccion_entrega, &metodo_pago, &total, &status); err != nil {
				continue
			}
			orders = append(orders, map[string]interface{}{
				"id":                id,
				"nombre":            nombre,
				"telefono":          telefono,
				"detalles_orden":    detalles_orden,
				"direccion_entrega": direccion_entrega,
				"metodo_pago":       metodo_pago,
				"total":             total,
				"status":            status,
			})
		}
		json.NewEncoder(w).Encode(orders)

	case "POST":
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
			return
		}
		
		nombre, _ := req["nombre"].(string)
		telefono, _ := req["telefono"].(string)
		detalles, _ := req["detalles_orden"].(string)
		direccion, _ := req["direccion_entrega"].(string)
		pago, _ := req["metodo_pago"].(string)
		total, _ := req["total"].(float64)

		id, err := InsertOrder(context.Background(), nombre, telefono, detalles, direccion, pago, total)
		if err != nil {
			http.Error(w, `{"error": "Failed to insert order"}`, http.StatusInternalServerError)
			return
		}

		req["id"] = id
		req["status"] = "PENDING"
		EmitOrder(req) // Notify kitchen monitor

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Order created", "id": id})

	case "PUT":
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
			return
		}
		
		idFloat, ok := req["id"].(float64)
		if !ok {
			http.Error(w, `{"error": "Missing or invalid id"}`, http.StatusBadRequest)
			return
		}
		status, _ := req["status"].(string)

		_, err := DB.Exec(context.Background(), "UPDATE orders SET status = $1 WHERE id = $2", status, int(idFloat))
		if err != nil {
			http.Error(w, `{"error": "Failed to update order"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Order updated successfully"})

	case "DELETE":
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error": "Invalid id"}`, http.StatusBadRequest)
			return
		}
		
		_, err = DB.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", id)
		if err != nil {
			http.Error(w, `{"error": "Failed to delete order"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Order deleted successfully"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleInventoryAPI(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if DB == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		rows, err := DB.Query(context.Background(), "SELECT MIN(id) as id, item, SUM(quantity) as quantity FROM inventory GROUP BY item ORDER BY item ASC")
		if err != nil {
			log.Printf("ERROR: GET /api/inventory failed: %v", err)
			http.Error(w, `{"error": "Failed to fetch inventory"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []map[string]interface{}
		for rows.Next() {
			var id, quantity int
			var item string
			if err := rows.Scan(&id, &item, &quantity); err == nil {
				items = append(items, map[string]interface{}{"id": id, "item": item, "quantity": quantity})
			}
		}
		json.NewEncoder(w).Encode(items)

	case "POST":
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
			return
		}
		
		item, _ := req["item"].(string)
		quantity, _ := req["quantity"].(float64)

		_, err := DB.Exec(context.Background(), "INSERT INTO inventory (item, quantity) VALUES ($1, $2)", item, int(quantity))
		if err != nil {
			http.Error(w, `{"error": "Failed to add inventory item"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "Item added"})

	case "PUT":
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
			return
		}
		
		idFloat, ok := req["id"].(float64)
		if !ok {
			http.Error(w, `{"error": "Missing or invalid id"}`, http.StatusBadRequest)
			return
		}
		quantity, _ := req["quantity"].(float64)

		_, err := DB.Exec(context.Background(), "UPDATE inventory SET quantity = $1 WHERE id = $2", int(quantity), int(idFloat))
		if err != nil {
			http.Error(w, `{"error": "Failed to update item"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Item updated"})
	
	case "DELETE":
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error": "Invalid id"}`, http.StatusBadRequest)
			return
		}
		
		_, err = DB.Exec(context.Background(), "DELETE FROM inventory WHERE id = $1", id)
		if err != nil {
			http.Error(w, `{"error": "Failed to delete item"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Item deleted"})
		
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if DB == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		var totalGanancias float64
		err := DB.QueryRow(context.Background(), "SELECT COALESCE(SUM(amount), 0) FROM earnings").Scan(&totalGanancias)
		if err != nil {
			log.Printf("ERROR: GET /api/dashboard failed to sum earnings: %v", err)
		}

		var pendingOrders int
		err = DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM orders WHERE status = 'PENDING'").Scan(&pendingOrders)
		if err != nil {
			log.Printf("ERROR: GET /api/dashboard failed to count orders: %v", err)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_ganancias": totalGanancias,
			"pending_orders":  pendingOrders,
		})
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
