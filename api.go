package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
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
		rows, err := DB.Query(context.Background(), "SELECT id, nombre, telefono, detalles_orden, direccion_entrega, metodo_pago, subtotal, tax, shipping, total, status, created_at FROM orders ORDER BY id DESC")
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
			var subtotal, tax, shipping, total float64
			var createdAt time.Time
			if err := rows.Scan(&id, &nombre, &telefono, &detalles_orden, &direccion_entrega, &metodo_pago, &subtotal, &tax, &shipping, &total, &status, &createdAt); err != nil {
				continue
			}
			orders = append(orders, map[string]interface{}{
				"id":                id,
				"nombre":            nombre,
				"telefono":          telefono,
				"detalles_orden":    detalles_orden,
				"direccion_entrega": direccion_entrega,
				"metodo_pago":       metodo_pago,
				"subtotal":          subtotal,
				"tax":               tax,
				"shipping":          shipping,
				"total":             total,
				"status":            status,
				"fecha_pedido":      createdAt.Format(time.RFC3339),
			})
		}
		if orders == nil {
			orders = make([]map[string]interface{}, 0)
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
		subtotal, _ := req["subtotal"].(float64)
		tax, _ := req["tax"].(float64)
		shipping, _ := req["shipping"].(float64)
		total, _ := req["total"].(float64)

		id, err := InsertOrder(context.Background(), nombre, telefono, detalles, direccion, pago, subtotal, tax, shipping, total, map[string]int{})
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
			var id int
			var quantity int64
			var item string
			if err := rows.Scan(&id, &item, &quantity); err == nil {
				items = append(items, map[string]interface{}{"id": id, "item": item, "quantity": quantity})
			} else {
				log.Printf("Scan error: %v", err)
			}
		}
		if items == nil {
			items = make([]map[string]interface{}, 0)
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
		
		item, ok := req["item"].(string)
		if !ok || item == "" {
			http.Error(w, `{"error": "Missing or invalid item name"}`, http.StatusBadRequest)
			return
		}
		quantity, _ := req["quantity"].(float64)

		// Consolidate: Delete all entries for this item and insert a fresh one with the updated exact quantity
		_, err := DB.Exec(context.Background(), "DELETE FROM inventory WHERE item = $1", item)
		if err == nil {
			_, err = DB.Exec(context.Background(), "INSERT INTO inventory (item, quantity) VALUES ($1, $2)", item, int(quantity))
		}
		
		if err != nil {
			http.Error(w, `{"error": "Failed to update item"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Item updated"})
	
	case "DELETE":
		item := r.URL.Query().Get("item")
		if item == "" {
			http.Error(w, `{"error": "Missing item name"}`, http.StatusBadRequest)
			return
		}
		
		_, err := DB.Exec(context.Background(), "DELETE FROM inventory WHERE item = $1", item)
		if err != nil {
			http.Error(w, `{"error": "Failed to delete item"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Item deleted"})
		
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

var lastGeminiError string

func HandleTestGeminiError(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	decision, err := CallGemini("test1", "hola")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	json.NewEncoder(w).Encode(decision)
}

func HandleSystemStatusAPI(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		whatsappStatus := GetWhatsAppStatus()
		errorsCount := len(GetSystemErrors())
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"whatsapp_status": whatsappStatus,
			"errors": GetSystemErrors(),
			"errors_count": errorsCount,
		})
	} else {
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
		var dailyGanancias, weeklyGanancias, monthlyGanancias, totalGanancias float64
		DB.QueryRow(context.Background(), `
			SELECT 
				COALESCE(SUM(amount), 0),
				COALESCE(SUM(amount) FILTER (WHERE created_at >= CURRENT_DATE), 0),
				COALESCE(SUM(amount) FILTER (WHERE created_at >= date_trunc('week', CURRENT_DATE)), 0),
				COALESCE(SUM(amount) FILTER (WHERE created_at >= date_trunc('month', CURRENT_DATE)), 0)
			FROM earnings
		`).Scan(&totalGanancias, &dailyGanancias, &weeklyGanancias, &monthlyGanancias)

		var pendingOrders int
		err := DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM orders WHERE status = 'PENDING'").Scan(&pendingOrders)
		if err != nil {
			log.Printf("ERROR: GET /api/dashboard failed to count orders: %v", err)
		}

		// Sales by payment method
		rows, err := DB.Query(context.Background(), "SELECT metodo_pago, SUM(total) FROM orders GROUP BY metodo_pago")
		var salesByPayment []map[string]interface{}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var mp string
				var am float64
				rows.Scan(&mp, &am)
				salesByPayment = append(salesByPayment, map[string]interface{}{"metodo_pago": mp, "amount": am})
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_ganancias": totalGanancias,
			"daily_ganancias": dailyGanancias,
			"weekly_ganancias": weeklyGanancias,
			"monthly_ganancias": monthlyGanancias,
			"pending_orders":  pendingOrders,
			"sales_by_payment": salesByPayment,
		})
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleTicketsAPI(w http.ResponseWriter, r *http.Request) {
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
		rows, err := DB.Query(context.Background(), "SELECT id, telefono, mensaje, status, created_at FROM support_tickets ORDER BY id DESC")
		if err != nil {
			log.Printf("ERROR: GET /api/tickets failed: %v", err)
			http.Error(w, `{"error": "Failed to fetch tickets"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var tickets []map[string]interface{}
		for rows.Next() {
			var id int
			var tel, mensaje, status string
			var created time.Time
			err := rows.Scan(&id, &tel, &mensaje, &status, &created)
			if err != nil {
				continue
			}
			tickets = append(tickets, map[string]interface{}{
				"id":         id,
				"telefono":   tel,
				"mensaje":    mensaje,
				"status":     status,
				"created_at": created.Format(time.RFC3339),
			})
		}
		if tickets == nil {
			tickets = make([]map[string]interface{}, 0)
		}
		json.NewEncoder(w).Encode(tickets)
		return
	} else if r.Method == "PUT" {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid body"}`, http.StatusBadRequest)
			return
		}
		idf, ok := req["id"].(float64)
		if !ok {
			http.Error(w, `{"error": "Invalid ID"}`, http.StatusBadRequest)
			return
		}
		status, _ := req["status"].(string)

		_, err := DB.Exec(context.Background(), "UPDATE support_tickets SET status=$1 WHERE id=$2", status, int(idf))
		if err != nil {
			http.Error(w, `{"error": "Update failed"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
		return
	}

	http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
}

func HandleAccountingAPI(w http.ResponseWriter, r *http.Request) {
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
		rows, err := DB.Query(context.Background(), "SELECT id, description, amount, category, created_at FROM expenses ORDER BY id DESC")
		if err != nil {
			log.Printf("ERROR: GET /api/accounting expenses: %v", err)
			http.Error(w, `{"error": "Failed to fetch expenses"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var expenses []map[string]interface{}
		for rows.Next() {
			var id int
			var description, category string
			var amount float64
			var created time.Time
			rows.Scan(&id, &description, &amount, &category, &created)
			expenses = append(expenses, map[string]interface{}{
				"id": id,
				"description": description,
				"amount": amount,
				"category": category,
				"created_at": created.Format(time.RFC3339),
			})
		}
		if expenses == nil {
			expenses = make([]map[string]interface{}, 0)
		}
		
		var totalExpenses float64
		DB.QueryRow(context.Background(), "SELECT COALESCE(SUM(amount), 0) FROM expenses").Scan(&totalExpenses)
		
		var totalEarnings float64
		DB.QueryRow(context.Background(), "SELECT COALESCE(SUM(amount), 0) FROM earnings").Scan(&totalEarnings)
		
		rowsCat, _ := DB.Query(context.Background(), "SELECT category, SUM(amount) FROM expenses GROUP BY category")
		var expensesByCategory []map[string]interface{}
		if rowsCat != nil {
			defer rowsCat.Close()
			for rowsCat.Next() {
				var cat string
				var am float64
				rowsCat.Scan(&cat, &am)
				expensesByCategory = append(expensesByCategory, map[string]interface{}{"category": cat, "amount": am})
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"expenses": expenses,
			"total_expenses": totalExpenses,
			"total_earnings": totalEarnings,
			"net_profit": totalEarnings - totalExpenses,
			"expenses_by_category": expensesByCategory,
		})
		return
	} else if r.Method == "POST" {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid body"}`, http.StatusBadRequest)
			return
		}
		desc, _ := req["description"].(string)
		amount, _ := req["amount"].(float64)
		cat, _ := req["category"].(string)
		
		_, err := DB.Exec(context.Background(), "INSERT INTO expenses (description, amount, category) VALUES ($1, $2, $3)", desc, amount, cat)
		if err != nil {
			http.Error(w, `{"error": "Insert failed"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}
	http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
}

func HandleMenuAPI(w http.ResponseWriter, r *http.Request) {
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
		rows, err := DB.Query(context.Background(), "SELECT id, name, description, price, category, is_active FROM menu_items ORDER BY id ASC")
		if err != nil {
			http.Error(w, `{"error": "Failed to fetch menu items"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []map[string]interface{}
		for rows.Next() {
			var id int
			var name, description, category string
			var price float64
			var isActive bool
			if err := rows.Scan(&id, &name, &description, &price, &category, &isActive); err == nil {
				items = append(items, map[string]interface{}{
					"id": id,
					"name": name,
					"description": description,
					"price": price,
					"category": category,
					"is_active": isActive,
				})
			}
		}
		if items == nil {
			items = make([]map[string]interface{}, 0)
		}
		json.NewEncoder(w).Encode(items)
		return
	} else if r.Method == "POST" {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid body"}`, http.StatusBadRequest)
			return
		}
		name, _ := req["name"].(string)
		desc, _ := req["description"].(string)
		price, _ := req["price"].(float64)
		cat, _ := req["category"].(string)
		
		_, err := DB.Exec(context.Background(), "INSERT INTO menu_items (name, description, price, category) VALUES ($1, $2, $3, $4)", name, desc, price, cat)
		if err != nil {
			http.Error(w, `{"error": "Insert failed"}`, http.StatusInternalServerError)
			return
		}
		UpdateGeminiPrompt() // Actualizar el prompt en memoria
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	} else if r.Method == "PUT" {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		idFloat, _ := req["id"].(float64)
		id := int(idFloat)

		if action, ok := req["action"].(string); ok && action == "toggle" {
			isActive, _ := req["is_active"].(bool)
			DB.Exec(context.Background(), "UPDATE menu_items SET is_active=$1 WHERE id=$2", isActive, id)
		} else {
		    name, _ := req["name"].(string)
		    desc, _ := req["description"].(string)
		    price, _ := req["price"].(float64)
		    cat, _ := req["category"].(string)
		    DB.Exec(context.Background(), "UPDATE menu_items SET name=$1, description=$2, price=$3, category=$4 WHERE id=$5", name, desc, price, cat, id)
		}
		UpdateGeminiPrompt()
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	} else if r.Method == "DELETE" {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		idFloat, _ := req["id"].(float64)
		
		DB.Exec(context.Background(), "DELETE FROM menu_items WHERE id=$1", int(idFloat))
		UpdateGeminiPrompt()
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
}
