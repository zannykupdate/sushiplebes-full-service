package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shopspring/decimal"
)

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bypass CORS preflight requests
		if r.Method == "OPTIONS" {
			next(w, r)
			return
		}

		user := AppConfig.AdminUser
		pass := AppConfig.AdminPass

		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted Area"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func main() {
	decimal.MarshalJSONWithoutQuotes = true
	LoadConfig()

	InitDB(AppConfig.DatabaseURL)
	if DB == nil {
		log.Fatal("ERROR CRÍTICO: No se puede operar sin conexión a PostgreSQL")
	}

	if err := DB.Ping(context.Background()); err == nil {
		log.Println("✅ Verificación de DB: Conexión y Ping exitosos antes de iniciar el servidor.")
	} else {
		log.Fatalf("CRITICAL ERROR: El Ping a la base de datos falló: %v\n", err)
	}

	port := AppConfig.Port

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	
	// Webhook remains unprotected (public for Meta)
	http.HandleFunc("/webhook", WebhookHandler)
	
	// Protected UI Routes
	http.HandleFunc("/monitor", basicAuth(HandleMonitorInterface))
	http.HandleFunc("/monitor/stream", basicAuth(HandleMonitorStream))
	http.HandleFunc("/admin", basicAuth(HandleAdminInterface))

	// CORS preflight global para /api/ en adelante
	http.HandleFunc("OPTIONS /api/", func(w http.ResponseWriter, r *http.Request) {
		if enableCors(&w, r) {
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})

	// Protected API Routes
	http.HandleFunc("GET /api/test_error", HandleTestGeminiError)
	http.HandleFunc("GET /api/orders", basicAuth(GetOrdersAPI))
	http.HandleFunc("POST /api/orders", basicAuth(CreateOrderAPI))
	http.HandleFunc("PUT /api/orders", basicAuth(UpdateOrderAPI))
	http.HandleFunc("DELETE /api/orders", basicAuth(DeleteOrderAPI))
	
	http.HandleFunc("GET /api/inventory", basicAuth(GetInventoryAPI))
	http.HandleFunc("POST /api/inventory", basicAuth(CreateInventoryAPI))
	http.HandleFunc("PUT /api/inventory", basicAuth(UpdateInventoryAPI))
	http.HandleFunc("DELETE /api/inventory", basicAuth(DeleteInventoryAPI))
	
	http.HandleFunc("GET /api/dashboard", basicAuth(GetDashboardAPI))
	http.HandleFunc("GET /api/system_status", basicAuth(GetSystemStatusAPI))
	
	http.HandleFunc("GET /api/tickets", basicAuth(GetTicketsAPI))
	http.HandleFunc("PUT /api/tickets", basicAuth(UpdateTicketAPI))
	
	http.HandleFunc("GET /api/accounting", basicAuth(GetAccountingAPI))
	http.HandleFunc("POST /api/accounting", basicAuth(CreateAccountingAPI))
	
	http.HandleFunc("GET /api/menu", basicAuth(GetMenuAPI))
	http.HandleFunc("POST /api/menu", basicAuth(CreateMenuAPI))
	http.HandleFunc("PUT /api/menu", basicAuth(UpdateMenuAPI))
	http.HandleFunc("DELETE /api/menu", basicAuth(DeleteMenuAPI))

	log.Println("⚡ Webhook escuchando en: http://localhost:" + port + "/webhook")
	log.Println("🖥️ Monitor disponible en: http://localhost:" + port + "/monitor")
	log.Println("📊 Panel Admin disponible en: http://localhost:" + port + "/admin")
	
	srv := &http.Server{Addr: "0.0.0.0:" + port, Handler: nil}
	
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("CRITICAL ERROR: El servidor colapsó: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
}
