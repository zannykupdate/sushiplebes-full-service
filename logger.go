package main

import (
	"log"
	"sync"
	"time"
)

type SystemError struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"` // e.g. "WHATSAPP_API", "GEMINI_API", "DATABASE"
	Message    string    `json:"message"`
	Details    string    `json:"details"`
	StatusCode int       `json:"status_code"`
}

var (
	errorHistory []SystemError
	errorMutex   sync.Mutex
	maxErrors    = 50
	
	// WhatsApp status cache
	lastWhatsAppStatus int = 200
	whatsAppStatusMutex sync.Mutex
)

func SetWhatsAppStatus(status int) {
	whatsAppStatusMutex.Lock()
	defer whatsAppStatusMutex.Unlock()
	lastWhatsAppStatus = status
}

func GetWhatsAppStatus() int {
	whatsAppStatusMutex.Lock()
	defer whatsAppStatusMutex.Unlock()
	return lastWhatsAppStatus
}

func LogSystemError(errType string, message string, details string, statusCode int) {
	log.Printf("[%s] %s: %s (Status: %d)", errType, message, details, statusCode)
	
	errorMutex.Lock()
	defer errorMutex.Unlock()

	errorHistory = append([]SystemError{{
		Timestamp:  time.Now(),
		Type:       errType,
		Message:    message,
		Details:    details,
		StatusCode: statusCode,
	}}, errorHistory...)

	if len(errorHistory) > maxErrors {
		errorHistory = errorHistory[:maxErrors]
	}
}

func GetSystemErrors() []SystemError {
	errorMutex.Lock()
	defer errorMutex.Unlock()
	
	// Return a copy
	res := make([]SystemError, len(errorHistory))
	copy(res, errorHistory)
	return res
}
