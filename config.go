package main

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	WhatsAppToken       string
	WhatsAppPhoneID     string
	WhatsAppVerifyToken string
	WhatsAppAppSecret   string
	GeminiAPIKey        string
	DatabaseURL         string
	FrontendURL         string
	MenuImageURL        string
	AdminUser           string
	AdminPass           string
	Port                string
}

var AppConfig Config

func LoadConfig() {
	AppConfig = Config{
		WhatsAppToken:       strings.TrimSpace(strings.Trim(os.Getenv("WHATSAPP_ACCESS_TOKEN"), "\"")),
		WhatsAppPhoneID:     strings.TrimSpace(strings.Trim(os.Getenv("WHATSAPP_PHONE_ID"), "\"")),
		WhatsAppVerifyToken: strings.TrimSpace(strings.Trim(os.Getenv("WHATSAPP_VERIFY_TOKEN"), "\"")),
		WhatsAppAppSecret:   strings.TrimSpace(strings.Trim(os.Getenv("WHATSAPP_APP_SECRET"), "\"")),
		GeminiAPIKey:        strings.TrimSpace(strings.Trim(os.Getenv("GEMINI_API_KEY"), "\"")),
		DatabaseURL:         strings.TrimSpace(strings.Trim(os.Getenv("DATABASE_URL"), "\"")),
		FrontendURL:         strings.TrimSpace(strings.Trim(os.Getenv("FRONTEND_URL"), "\"")),
		MenuImageURL:        strings.TrimSpace(strings.Trim(os.Getenv("MENU_IMAGE_URL"), "\"")),
		AdminUser:           strings.TrimSpace(strings.Trim(os.Getenv("ADMIN_USER"), "\"")),
		AdminPass:           strings.TrimSpace(strings.Trim(os.Getenv("ADMIN_PASS"), "\"")),
		Port:                strings.TrimSpace(strings.Trim(os.Getenv("PORT"), "\"")),
	}

	if AppConfig.AdminUser == "" || AppConfig.AdminPass == "" {
		log.Fatal("CRITICAL ERROR: Variables de entorno ADMIN_USER y ADMIN_PASS son obligatorias (Fail-Fast).")
	}
	if AppConfig.DatabaseURL == "" {
		log.Fatal("CRITICAL ERROR: No se puede operar sin base de datos (DATABASE_URL no definida)")
	}
	if AppConfig.Port == "" {
		AppConfig.Port = "3000"
	}
}
