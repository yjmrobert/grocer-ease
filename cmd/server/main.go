package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yjmrobert/grocer-ease/internal/handler"
	"github.com/yjmrobert/grocer-ease/internal/provider"
	"github.com/yjmrobert/grocer-ease/internal/service"
	"github.com/yjmrobert/grocer-ease/internal/store"
)

func main() {
	port := envOrDefault("PORT", "8080")
	dbPath := envOrDefault("DB_PATH", "grocer-ease.db")
	postalCode := envOrDefault("POSTAL_CODE", "M5V")

	db, err := store.NewDB(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	listStore := store.NewListStore(db)
	cacheStore := store.NewPriceCacheStore(db, 6*time.Hour)
	settingsStore := store.NewSettingsStore(db)

	// Seed postal code from env if not already in DB
	if settingsStore.Get("postal_code", "") == "" && postalCode != "" {
		settingsStore.Set("postal_code", postalCode)
	}
	if settingsStore.Get("trip_penalty", "") == "" {
		settingsStore.Set("trip_penalty", "5")
	}

	providers := buildProviders(postalCode)
	priceService := service.NewPriceService(providers, cacheStore)

	router := handler.NewRouter(listStore, priceService, cacheStore, settingsStore)

	if len(providers) == 0 {
		log.Println("WARNING: No price providers configured. Set POSTAL_CODE env var.")
	} else {
		log.Printf("Configured %d price provider(s):", len(providers))
		for _, p := range providers {
			log.Printf("  - %s", p.Store())
		}
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Grocer-Ease starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func buildProviders(postalCode string) []provider.PriceProvider {
	var providers []provider.PriceProvider

	// Flipp providers — always enabled if POSTAL_CODE is set
	if postalCode != "" {
		providers = append(providers,
			provider.NewFlippProvider("Walmart", postalCode),
			provider.NewFlippProvider("Loblaws", postalCode),
			provider.NewFlippProvider("Maxi", postalCode),
			provider.NewFlippProvider("No Frills", postalCode),
			provider.NewFlippProvider("Metro", postalCode),
			provider.NewFlippProvider("FreshCo", postalCode),
			provider.NewFlippProvider("Sobeys", postalCode),
			provider.NewFlippProvider("Food Basics", postalCode),
		)
	}

	// Loblaws/PC Express API — requires API key
	loblawsKey := os.Getenv("LOBLAWS_API_KEY")
	if loblawsKey != "" {
		loblawsStoreID := envOrDefault("LOBLAWS_STORE_ID", "1001")
		maxiStoreID := envOrDefault("MAXI_STORE_ID", "")

		providers = append(providers,
			provider.NewLoblawsProvider(provider.BannerLoblaws, "Loblaws (Direct)", loblawsStoreID, loblawsKey),
		)
		if maxiStoreID != "" {
			providers = append(providers,
				provider.NewLoblawsProvider(provider.BannerMaxi, "Maxi (Direct)", maxiStoreID, loblawsKey),
			)
		}
	}

	// Walmart.ca direct scraping — opt-in because CloudFlare often blocks it
	if os.Getenv("ENABLE_WALMART") == "true" {
		providers = append(providers, provider.NewWalmartProvider())
	}

	return providers
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
