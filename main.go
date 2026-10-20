package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"convention-management-system/database"
	"convention-management-system/routes"
	"convention-management-system/services"
)

func main() {

	err := godotenv.Load()

	if err != nil {
		log.Println("Warning: .env file not found:", err)
	}

	// Initialize database
	if err := database.Init(); err != nil {
		log.Fatal(err)
	}

	// Run database migrations
	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	if err := database.SeedAdmin(); err != nil {
		log.Fatal(err)
	}

	if err := database.SeedConvention(); err != nil {
		log.Fatal(err)
	}

	// Non-destructive startup media durability audit & synchronization
	services.AutoSyncEmbeddedMedia(database.DB)

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
