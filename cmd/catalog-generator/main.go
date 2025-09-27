package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/piligrim/pushkinlib/internal/catalog"
)

func main() {
	// Command line flags
	var (
		booksDir       = flag.String("books", "./sample-data/books", "Directory containing book files")
		outputDir      = flag.String("output", "./sample-data", "Output directory for generated files")
		catalogName    = flag.String("name", "generated_catalog", "Name of the catalog")
		archivePrefix  = flag.String("prefix", "books", "Prefix for generated ZIP archives")
		maxBooks       = flag.Int("max-books", 1000, "Maximum books per ZIP archive")
		includeFormats = flag.String("formats", ".fb2,.zip,.epub", "Comma-separated list of file formats to include")
		help           = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Validate input
	if _, err := os.Stat(*booksDir); os.IsNotExist(err) {
		log.Fatalf("Books directory does not exist: %s", *booksDir)
	}

	// Parse formats
	formats := strings.Split(*includeFormats, ",")
	for i, format := range formats {
		formats[i] = strings.TrimSpace(format)
		if !strings.HasPrefix(formats[i], ".") {
			formats[i] = "." + formats[i]
		}
	}

	// Create generator
	generator := catalog.NewGenerator()

	// Prepare options
	opts := catalog.GenerateOptions{
		BooksDir:       *booksDir,
		OutputDir:      *outputDir,
		CatalogName:    *catalogName,
		ArchivePrefix:  *archivePrefix,
		MaxBooksPerZip: *maxBooks,
		IncludeFormats: formats,
	}

	// Show configuration
	fmt.Println("=== Catalog Generator ===")
	fmt.Printf("Books directory: %s\n", opts.BooksDir)
	fmt.Printf("Output directory: %s\n", opts.OutputDir)
	fmt.Printf("Catalog name: %s\n", opts.CatalogName)
	fmt.Printf("Archive prefix: %s\n", opts.ArchivePrefix)
	fmt.Printf("Max books per archive: %d\n", opts.MaxBooksPerZip)
	fmt.Printf("Include formats: %s\n", strings.Join(opts.IncludeFormats, ", "))
	fmt.Println()

	// Generate catalog
	result, err := generator.Generate(opts)
	if err != nil {
		log.Fatalf("Failed to generate catalog: %v", err)
	}

	// Show results
	fmt.Println("=== Generation Results ===")
	fmt.Printf("Total books found: %d\n", result.TotalBooks)
	fmt.Printf("Successfully processed: %d\n", result.ProcessedBooks)
	fmt.Printf("Skipped (errors): %d\n", result.SkippedBooks)
	fmt.Printf("Generated archives: %d\n", len(result.GeneratedZips))
	fmt.Printf("Processing time: %v\n", result.ProcessingTime)
	fmt.Printf("INPX file: %s\n", result.INPXPath)
	fmt.Println()

	if len(result.GeneratedZips) > 0 {
		fmt.Println("Generated archives:")
		for i, zipPath := range result.GeneratedZips {
			fmt.Printf("  %d. %s\n", i+1, filepath.Base(zipPath))
		}
		fmt.Println()
	}

	// Show collection info
	fmt.Println("=== Collection Info ===")
	fmt.Printf("Name: %s\n", result.CollectionInfo.Name)
	fmt.Printf("Version: %s\n", result.CollectionInfo.Version)
	fmt.Printf("Description: %s\n", result.CollectionInfo.Description)
	fmt.Printf("Date: %s\n", result.CollectionInfo.Date)
	fmt.Println()

	// Show errors if any
	if len(result.Errors) > 0 {
		fmt.Printf("=== Errors (%d) ===\n", len(result.Errors))
		for i, err := range result.Errors {
			if i < 10 { // Show only first 10 errors
				fmt.Printf("  %d. %v\n", i+1, err)
			}
		}
		if len(result.Errors) > 10 {
			fmt.Printf("  ... and %d more errors\n", len(result.Errors)-10)
		}
		fmt.Println()
	}

	// Usage instructions
	fmt.Println("=== Usage Instructions ===")
	fmt.Printf("1. Copy the generated INPX file to your server:\n")
	fmt.Printf("   cp %s /path/to/your/server/\n", result.INPXPath)
	fmt.Println()
	fmt.Printf("2. Copy the generated archives to your books directory:\n")
	for _, zipPath := range result.GeneratedZips {
		fmt.Printf("   cp %s /path/to/your/books/\n", zipPath)
	}
	fmt.Println()
	fmt.Printf("3. Update your .env file:\n")
	fmt.Printf("   INPX_PATH=/path/to/%s\n", filepath.Base(result.INPXPath))
	fmt.Printf("   BOOKS_DIR=/path/to/your/books/\n")
	fmt.Println()

	// Test command
	fmt.Println("=== Test Command ===")
	fmt.Printf("To test with the generated catalog:\n")
	fmt.Printf("INPX_PATH=%s BOOKS_DIR=%s ./pushkinlib\n",
		result.INPXPath, filepath.Dir(result.GeneratedZips[0]))

	fmt.Println("\n✅ Catalog generation completed successfully!")
}

func showHelp() {
	fmt.Println("Catalog Generator - Creates INPX catalog from book files")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  catalog-generator [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Generate catalog from default books directory")
	fmt.Println("  catalog-generator")
	fmt.Println()
	fmt.Println("  # Generate catalog with custom settings")
	fmt.Println("  catalog-generator -books=/home/user/books -name=my_library -max-books=500")
	fmt.Println()
	fmt.Println("  # Include only FB2 files")
	fmt.Println("  catalog-generator -formats=.fb2")
	fmt.Println()
	fmt.Println("Supported formats:")
	fmt.Println("  .fb2  - FictionBook 2.0 files")
	fmt.Println("  .zip  - ZIP archives containing FB2 files")
	fmt.Println("  .epub - EPUB files (basic support)")
	fmt.Println()
}