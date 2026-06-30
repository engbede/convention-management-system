package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"convention-management-system/database"
	"convention-management-system/handlers"
	"convention-management-system/routes"
)

func main() {

	// Load templates
	handlers.Templates = template.Must(
		template.Must(
			template.ParseGlob("templates/*.html"),
		).ParseGlob("templates/partials/*.html"),
	)

	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatal(err)
	}

	// Run database migrations
	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	// Seed default admin
	if err := database.SeedAdmin(); err != nil {
		log.Fatal(err)
	}

	// Register routes
	mux := http.NewServeMux()
	routes.RegisterRoutes(mux)

	// Use environment PORT if available
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	log.Printf("Server started on :%s", port)

	log.Fatal(
		http.ListenAndServe(
			":"+port,
			mux,
		),
	)
}
