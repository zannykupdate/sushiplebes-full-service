package main

import (
	"context"
	"log"
	"net/http"
	"os"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
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

	http.HandleFunc("/webhook", WebhookHandler)
	http.HandleFunc("/monitor", HandleMonitorInterface)
	http.HandleFunc("/monitor/stream", HandleMonitorStream)
	http.HandleFunc("/api/orders", HandleOrdersAPI)
	http.HandleFunc("/api/inventory", HandleInventoryAPI)
	http.HandleFunc("/admin", HandleAdminInterface)

	log.Println("⚡ Webhook escuchando en: http://localhost:" + port + "/webhook")
	log.Println("🖥️ Monitor disponible en: http://localhost:" + port + "/monitor")
	log.Println("📊 Panel Admin disponible en: http://localhost:" + port + "/admin")
	
	err := http.ListenAndServe("0.0.0.0:"+port, nil)
	if err != nil {
		log.Fatalf("CRITICAL ERROR: El servidor colapsó: %v\n", err)
	}
}
