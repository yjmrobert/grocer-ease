package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/yjmrobert/grocer-ease/internal/handler"
	"github.com/yjmrobert/grocer-ease/internal/provider"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "grocer-ease.db"
	}

	db, err := store.NewDB(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	listStore := store.NewListStore(db)

	// Price providers will be added in Phase 2+
	var providers []provider.PriceProvider

	router := handler.NewRouter(listStore, providers)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Grocer-Ease starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
