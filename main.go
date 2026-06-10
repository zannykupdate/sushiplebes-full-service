package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
)

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bypass CORS preflight requests
		if r.Method == "OPTIONS" {
			next(w, r)
			return
		}

		user := strings.TrimSpace(strings.Trim(os.Getenv("ADMIN_USER"), "\""))
		pass := strings.TrimSpace(strings.Trim(os.Getenv("ADMIN_PASS"), "\""))
		
		// If credentials are not set in the environment, fallback to a default or block
		if user == "" || pass == "" {
			user = "admin"
			pass = "admin123" // Fallback default if not defined to ensure protection
		}

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
	databaseURL := strings.TrimSpace(strings.Trim(os.Getenv("DATABASE_URL"), "\""))
	if databaseURL != "" {
		InitDB(databaseURL)
	} else {
		log.Println("WARNING: DATABASE_URL no provista")
	}

	if DB != nil {
		if err := DB.Ping(context.Background()); err == nil {
			log.Println("✅ Verificación de DB: Conexión y Ping exitosos antes de iniciar el servidor.")
		} else {
			log.Printf("⚠️ Verificación de DB: El Ping a la base de datos falló: %v\n", err)
		}
	} else {
		log.Println("⚠️ Verificación de DB: Operando sin conexión a base de datos activa (DB es nil).")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

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

	// Protected API Routes
	http.HandleFunc("/api/test_error", HandleTestGeminiError)
	http.HandleFunc("/api/orders", basicAuth(HandleOrdersAPI))
	http.HandleFunc("/api/inventory", basicAuth(HandleInventoryAPI))
	http.HandleFunc("/api/dashboard", basicAuth(HandleDashboardAPI))
	http.HandleFunc("/api/system_status", basicAuth(HandleSystemStatusAPI))

	log.Println("⚡ Webhook escuchando en: http://localhost:" + port + "/webhook")
	log.Println("🖥️ Monitor disponible en: http://localhost:" + port + "/monitor")
	log.Println("📊 Panel Admin disponible en: http://localhost:" + port + "/admin")
	
	err := http.ListenAndServe("0.0.0.0:"+port, nil)
	if err != nil {
		log.Fatalf("CRITICAL ERROR: El servidor colapsó: %v\n", err)
	}
}
