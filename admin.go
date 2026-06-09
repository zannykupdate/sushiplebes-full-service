package main

import (
	"log"
	"net/http"
	"text/template"
)

// HandleAdminInterface serves the administrative dashboard panel HTML.
func HandleAdminInterface(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		log.Printf("ERROR: Could not load admin template: %v", err)
		http.Error(w, "Error loading admin template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}
