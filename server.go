package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// 1. Inicializar la Base de Datos al arrancar
	InitDB()
	if DB != nil {
		defer DB.Close()
	}

	// 2. Registrar los Enpoints del ecosistema
	http.HandleFunc("/webhook", WebhookHandler)             // Recibe datos de Meta y dispara el Bot (webhook.go y bot.go)
	http.HandleFunc("/monitor", HandleMonitorInterface)     // Renderiza la GUI de cocina (monitor.go)
	http.HandleFunc("/monitor/stream", HandleMonitorStream) // Transmisión SSE asíncrona a la cocina (monitor.go)

	// 3. Obtener el puerto, manejar fallbacks
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 4. Iniciar el servidor
	log.Printf("🍣 Servidor principal de SUSHI LOSPLEBES unificado e iniciado en el puerto: %s\n", port)
	log.Println("⚡ Webhook escuchando en:  http://localhost:" + port + "/webhook")
	log.Println("🖥️ Monitor disponible en: http://localhost:" + port + "/monitor")
	
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("CRITICAL ERROR: El servidor colapsó: %v\n", err)
	}
}
