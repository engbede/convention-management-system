package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"convention-management-system/database"
	"convention-management-system/handlers"
	"convention-management-system/routes"
)

func main() {

	err := godotenv.Load()

	if err != nil {
		log.Println("Warning: .env file not found:", err)
	}
	// Load templates
	handlers.Templates = template.Must(
		template.Must(
			template.ParseGlob("templates/*.html"),
		).ParseGlob("templates/partials/*.html"),
	)

	// Initialize database
	if err := database.Init(); err != nil {
		log.Fatal(err)
	}

	if err := database.SeedAdmin(); err != nil {
		log.Fatal(err)
	}

	if err := database.SeedConvention(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux)

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
