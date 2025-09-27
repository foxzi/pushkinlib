package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/piligrim/pushkinlib/internal/api"
	"github.com/piligrim/pushkinlib/internal/config"
	"github.com/piligrim/pushkinlib/internal/inpx"
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
		if err := importINPX(cfg, repo); err != nil {
			log.Fatalf("Failed to import INPX: %v", err)
		}
	} else {
		fmt.Printf("Database contains %d books\n", searchResult.Total)
	}

	// Setup API routes
	handlers := api.NewHandlers(repo, cfg.BooksDir)
	router := api.SetupRoutes(handlers)

	// Setup OPDS routes
	baseURL := fmt.Sprintf("http://localhost:%s", cfg.Port)
	opdsHandler := opds.NewHandler(repo, baseURL, cfg.CatalogTitle)
	api.SetupOPDSRoutes(router, opdsHandler)

	// Setup HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Starting HTTP server on port %s\n", cfg.Port)
		fmt.Printf("Web interface: http://localhost:%s/\n", cfg.Port)
		fmt.Printf("API available at: http://localhost:%s/api/v1/books\n", cfg.Port)
		fmt.Printf("OPDS catalog: http://localhost:%s/opds\n", cfg.Port)
		fmt.Printf("Health check at: http://localhost:%s/health\n", cfg.Port)

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

func importINPX(cfg *config.Config, repo *storage.Repository) error {
	// Check if INPX file exists
	if _, err := os.Stat(cfg.INPXPath); os.IsNotExist(err) {
		return fmt.Errorf("INPX file not found: %s", cfg.INPXPath)
	}

	// Parse INPX
	fmt.Println("Parsing INPX file...")
	parser := inpx.NewParser()
	books, collectionInfo, err := parser.ParseINPX(cfg.INPXPath)
	if err != nil {
		return fmt.Errorf("failed to parse INPX: %w", err)
	}

	fmt.Printf("Found %d books in collection: %s\n", len(books), collectionInfo.Name)

	// Limit import for testing (remove this in production)
	if len(books) > 1000 {
		fmt.Printf("Limiting import to first 1000 books for testing...\n")
		books = books[:1000]
	}

	// Clear existing data
	fmt.Println("Clearing existing data...")
	if err := repo.ClearAllBooks(); err != nil {
		return fmt.Errorf("failed to clear existing data: %w", err)
	}

	// Insert books into database
	fmt.Println("Inserting books into database...")
	if err := repo.InsertBooks(books); err != nil {
		return fmt.Errorf("failed to insert books: %w", err)
	}

	fmt.Printf("Successfully imported %d books!\n", len(books))
	return nil
}