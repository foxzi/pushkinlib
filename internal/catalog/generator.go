package catalog

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/piligrim/pushkinlib/internal/metadata"
)

// Generator creates INPX catalogs from book files
type Generator struct {
	extractor *metadata.Extractor
}

// NewGenerator creates a new catalog generator
func NewGenerator() *Generator {
	return &Generator{
		extractor: metadata.NewExtractor(),
	}
}

// GenerateOptions contains options for catalog generation
type GenerateOptions struct {
	BooksDir        string
	OutputDir       string
	CatalogName     string
	ArchivePrefix   string
	MaxBooksPerZip  int
	IncludeFormats  []string
}

// GenerationResult contains results of catalog generation
type GenerationResult struct {
	TotalBooks      int
	ProcessedBooks  int
	SkippedBooks    int
	GeneratedZips   []string
	INPXPath        string
	CollectionInfo  CollectionInfo
	ProcessingTime  time.Duration
	Errors          []error
}

// CollectionInfo represents collection metadata
type CollectionInfo struct {
	Name        string
	Version     string
	Description string
	Date        string
}

// Generate creates INPX catalog from books directory
func (g *Generator) Generate(opts GenerateOptions) (*GenerationResult, error) {
	startTime := time.Now()

	// Set defaults
	if opts.MaxBooksPerZip == 0 {
		opts.MaxBooksPerZip = 1000
	}
	if len(opts.IncludeFormats) == 0 {
		opts.IncludeFormats = []string{".fb2", ".zip", ".epub"}
	}
	if opts.ArchivePrefix == "" {
		opts.ArchivePrefix = "books"
	}

	result := &GenerationResult{
		ProcessingTime: time.Since(startTime),
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Scan books directory
	fmt.Printf("Scanning books directory: %s\n", opts.BooksDir)
	bookFiles, err := g.scanBooksDirectory(opts.BooksDir, opts.IncludeFormats)
	if err != nil {
		return nil, fmt.Errorf("failed to scan books directory: %w", err)
	}

	result.TotalBooks = len(bookFiles)
	fmt.Printf("Found %d book files\n", result.TotalBooks)

	if result.TotalBooks == 0 {
		return result, nil
	}

	// Extract metadata from all books
	fmt.Println("Extracting metadata...")
	var allMetadata []*metadata.BookMetadata
	for i, filePath := range bookFiles {
		if i%100 == 0 && i > 0 {
			fmt.Printf("Processed %d/%d files...\n", i, result.TotalBooks)
		}

		meta, err := g.extractor.ExtractFromFile(filePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to extract metadata from %s: %w", filePath, err))
			result.SkippedBooks++
			continue
		}

		allMetadata = append(allMetadata, meta)
		result.ProcessedBooks++
	}

	fmt.Printf("Successfully extracted metadata from %d books\n", result.ProcessedBooks)

	// Create book archives
	fmt.Println("Creating book archives...")
	zipPaths, err := g.createBookArchives(allMetadata, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create book archives: %w", err)
	}

	result.GeneratedZips = zipPaths

	// Generate INPX
	fmt.Println("Generating INPX file...")
	inpxPath, collectionInfo, err := g.generateINPX(allMetadata, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate INPX: %w", err)
	}

	result.INPXPath = inpxPath
	result.CollectionInfo = collectionInfo
	result.ProcessingTime = time.Since(startTime)

	fmt.Printf("Catalog generation completed in %v\n", result.ProcessingTime)
	fmt.Printf("Generated INPX: %s\n", inpxPath)
	fmt.Printf("Generated %d archives\n", len(zipPaths))

	return result, nil
}

// scanBooksDirectory scans directory for book files
func (g *Generator) scanBooksDirectory(dir string, includeFormats []string) ([]string, error) {
	var bookFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, format := range includeFormats {
			if ext == format {
				bookFiles = append(bookFiles, path)
				break
			}
		}

		return nil
	})

	return bookFiles, err
}

// createBookArchives creates ZIP archives with books
func (g *Generator) createBookArchives(allMetadata []*metadata.BookMetadata, opts GenerateOptions) ([]string, error) {
	var zipPaths []string

	// Sort metadata by title for consistent ordering
	sort.Slice(allMetadata, func(i, j int) bool {
		return allMetadata[i].Title < allMetadata[j].Title
	})

	currentZip := 0
	currentBooks := 0

	var currentZipWriter *zip.Writer
	var currentZipFile *os.File
	var currentZipPath string

	for i, meta := range allMetadata {
		// Start new archive if needed
		if currentBooks == 0 || currentBooks >= opts.MaxBooksPerZip {
			// Close previous archive
			if currentZipWriter != nil {
				currentZipWriter.Close()
				currentZipFile.Close()
			}

			// Create new archive
			currentZip++
			currentZipPath = filepath.Join(opts.OutputDir, fmt.Sprintf("%s-%06d.zip", opts.ArchivePrefix, currentZip))

			var err error
			currentZipFile, err = os.Create(currentZipPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create zip file %s: %w", currentZipPath, err)
			}

			currentZipWriter = zip.NewWriter(currentZipFile)
			zipPaths = append(zipPaths, currentZipPath)
			currentBooks = 0

			fmt.Printf("Creating archive %d: %s\n", currentZip, filepath.Base(currentZipPath))
		}

		// Add book to archive
		bookID := fmt.Sprintf("%06d", i+1)
		fileName := bookID + "." + meta.Format

		// Update metadata with archive info
		meta.ID = bookID
		meta.ArchivePath = strings.TrimSuffix(filepath.Base(currentZipPath), ".zip")
		meta.FileNum = bookID

		err := g.addBookToZip(currentZipWriter, meta, fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to add book to zip: %w", err)
		}

		currentBooks++
	}

	// Close last archive
	if currentZipWriter != nil {
		currentZipWriter.Close()
		currentZipFile.Close()
	}

	return zipPaths, nil
}

// addBookToZip adds a book file to ZIP archive
func (g *Generator) addBookToZip(zipWriter *zip.Writer, meta *metadata.BookMetadata, fileName string) error {
	// Open source file
	sourceFile, err := os.Open(meta.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create entry in ZIP
	zipEntry, err := zipWriter.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// Copy file content
	_, err = io.Copy(zipEntry, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// generateINPX creates INPX file with all metadata
func (g *Generator) generateINPX(allMetadata []*metadata.BookMetadata, opts GenerateOptions) (string, CollectionInfo, error) {
	now := time.Now()
	dateStr := now.Format("2006-01-02")

	collectionInfo := CollectionInfo{
		Name:        fmt.Sprintf("%s - %s", opts.CatalogName, dateStr),
		Version:     dateStr,
		Description: fmt.Sprintf("Generated catalog of %d books", len(allMetadata)),
		Date:        dateStr,
	}

	inpxPath := filepath.Join(opts.OutputDir, opts.CatalogName+".inpx")

	// Create INPX zip file
	inpxFile, err := os.Create(inpxPath)
	if err != nil {
		return "", collectionInfo, fmt.Errorf("failed to create INPX file: %w", err)
	}
	defer inpxFile.Close()

	zipWriter := zip.NewWriter(inpxFile)
	defer zipWriter.Close()

	// Group books by archive
	archiveBooks := make(map[string][]*metadata.BookMetadata)
	for _, meta := range allMetadata {
		archiveBooks[meta.ArchivePath] = append(archiveBooks[meta.ArchivePath], meta)
	}

	// Create INP files for each archive
	for archiveName, books := range archiveBooks {
		inpFileName := archiveName + ".inp"
		inpWriter, err := zipWriter.Create(inpFileName)
		if err != nil {
			return "", collectionInfo, fmt.Errorf("failed to create INP file: %w", err)
		}

		for _, meta := range books {
			line := g.formatINPLine(meta)
			if _, err := inpWriter.Write([]byte(line + "\n")); err != nil {
				return "", collectionInfo, fmt.Errorf("failed to write INP line: %w", err)
			}
		}
	}

	// Create collection.info
	infoWriter, err := zipWriter.Create("collection.info")
	if err != nil {
		return "", collectionInfo, fmt.Errorf("failed to create collection.info: %w", err)
	}

	infoContent := fmt.Sprintf("%s\n%s\n65536\n%s\n",
		collectionInfo.Name,
		collectionInfo.Version,
		collectionInfo.Description)

	if _, err := infoWriter.Write([]byte(infoContent)); err != nil {
		return "", collectionInfo, fmt.Errorf("failed to write collection.info: %w", err)
	}

	// Create version.info
	versionWriter, err := zipWriter.Create("version.info")
	if err != nil {
		return "", collectionInfo, fmt.Errorf("failed to create version.info: %w", err)
	}

	if _, err := versionWriter.Write([]byte(collectionInfo.Version + "\n")); err != nil {
		return "", collectionInfo, fmt.Errorf("failed to write version.info: %w", err)
	}

	return inpxPath, collectionInfo, nil
}

// formatINPLine formats book metadata as INP line
func (g *Generator) formatINPLine(meta *metadata.BookMetadata) string {
	// AUTHOR\x04GENRE\x04TITLE\x04SERIES\x04SERIES_NUM\x04BOOK_ID\x04SIZE\x04ARCHIVE_PATH\x04FILE_NUM\x04FORMAT\x04DATE\x04LANG\x04RATING\x04ANNOTATION\x04

	fields := []string{
		strings.Join(meta.Authors, ","),                 // AUTHOR
		strings.Join(meta.Genres, ","),                  // GENRE
		meta.Title,                                      // TITLE
		meta.Series,                                     // SERIES
		strconv.Itoa(meta.SeriesNum),                   // SERIES_NUM
		meta.ID,                                         // BOOK_ID
		strconv.FormatInt(meta.FileSize, 10),           // SIZE
		meta.ArchivePath,                               // ARCHIVE_PATH
		meta.FileNum,                                   // FILE_NUM
		meta.Format,                                    // FORMAT
		meta.Date.Format("2006-01-02"),                // DATE
		meta.Language,                                  // LANG
		"0",                                            // RATING (default)
		meta.Annotation,                                // ANNOTATION
		"",                                             // End marker
	}

	return strings.Join(fields, "\x04")
}