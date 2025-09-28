package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/piligrim/pushkinlib/internal/api"
	"github.com/piligrim/pushkinlib/internal/config"
	"github.com/piligrim/pushkinlib/internal/indexer"
	"github.com/piligrim/pushkinlib/internal/opds"
	"github.com/piligrim/pushkinlib/internal/storage"
)

func main() {
	cfg := config.LoadConfig()

	fmt.Printf("Pushkinlib starting...\n")
	fmt.Printf("Port: %s\n", cfg.Port)
	fmt.Printf("INPX Path: %s\n", cfg.INPXPath)
	fmt.Printf("Database: %s\n", cfg.DatabasePath)

	// Initialize database
	db, err := storage.NewDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewRepository(db)

	// Check if database has data
	searchResult, err := repo.SearchBooks(storage.BookFilter{Limit: 1})
	if err != nil {
		log.Fatalf("Failed to check database: %v", err)
	}

	if searchResult.Total == 0 {
		fmt.Println("Database is empty, importing INPX data...")
		result, err := indexer.ReindexFromINPX(repo, cfg.INPXPath)
		if err != nil {
			log.Fatalf("Failed to import INPX: %v", err)
		}
		collectionName := "INPX"
		if result.Collection != nil && result.Collection.Name != "" {
			collectionName = result.Collection.Name
		}
		fmt.Printf("Imported %d books from %s in %s\n", result.Imported, collectionName, result.Duration.Truncate(time.Millisecond))
	} else {
		fmt.Printf("Database contains %d books\n", searchResult.Total)
	}

	// Setup API routes
	handlers := api.NewHandlers(repo, cfg.BooksDir, cfg.INPXPath)
	router := api.SetupRoutes(handlers)

	// Load genre translations for OPDS
	genreNames, err := opds.LoadGenreNames(cfg.GenresCSVPath)
	if err != nil {
		log.Printf("Failed to load genre translations from %s: %v", cfg.GenresCSVPath, err)
	}

	// Setup OPDS routes
	baseURL := strings.TrimSpace(cfg.PublicBaseURL)
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", cfg.Port)
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	opdsHandler := opds.NewHandler(repo, baseURL, cfg.CatalogTitle, genreNames)
	api.SetupOPDSRoutes(router, opdsHandler)

	// Setup HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Starting HTTP server on port %s\n", cfg.Port)
		fmt.Printf("Public base URL: %s\n", baseURL)
		fmt.Printf("Web interface: %s/\n", baseURL)
		fmt.Printf("API available at: %s/api/v1/books\n", baseURL)
		fmt.Printf("OPDS catalog: %s/opds\n", baseURL)
		fmt.Printf("Health check at: %s/health\n", baseURL)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	fmt.Println("Server stopped")
}
