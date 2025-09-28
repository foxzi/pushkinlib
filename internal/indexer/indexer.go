package indexer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/piligrim/pushkinlib/internal/inpx"
	"github.com/piligrim/pushkinlib/internal/storage"
)

var (
	// ErrINPXPathEmpty indicates that no INPX path was provided.
	ErrINPXPathEmpty = errors.New("inpx path is empty")
	// ErrINPXNotFound indicates that the provided INPX file does not exist.
	ErrINPXNotFound = errors.New("inpx file not found")
)

// Result contains statistics about a reindex operation.
type Result struct {
	Imported       int
	Collection     *inpx.CollectionInfo
	Duration       time.Duration
	ParseDuration  time.Duration
	ClearDuration  time.Duration
	InsertDuration time.Duration
}

// ReindexFromINPX clears all existing data and loads books from the provided INPX file.
func ReindexFromINPX(repo *storage.Repository, inpxPath string) (*Result, error) {
	if inpxPath == "" {
		return nil, ErrINPXPathEmpty
	}

	if _, err := os.Stat(inpxPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrINPXNotFound, inpxPath)
		}
		return nil, fmt.Errorf("failed to access inpx file: %w", err)
	}

	parser := inpx.NewParser()
	totalStart := time.Now()

	log.Printf("Reindex: parsing INPX file %s", inpxPath)
	parseStart := time.Now()
	books, collectionInfo, err := parser.ParseINPX(inpxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inpx: %w", err)
	}
	parseDuration := time.Since(parseStart)
	log.Printf("Reindex: parsed %d books in %s", len(books), parseDuration.Truncate(time.Millisecond))

	log.Printf("Reindex: clearing existing data")
	clearStart := time.Now()
	if err := repo.ClearAllBooks(); err != nil {
		return nil, fmt.Errorf("failed to clear existing data: %w", err)
	}
	clearDuration := time.Since(clearStart)
	log.Printf("Reindex: cleared existing data in %s", clearDuration.Truncate(time.Millisecond))

	log.Printf("Reindex: inserting books into database")
	insertStart := time.Now()
	if err := repo.InsertBooks(books); err != nil {
		return nil, fmt.Errorf("failed to insert books: %w", err)
	}
	insertDuration := time.Since(insertStart)
	log.Printf("Reindex: inserted books in %s", insertDuration.Truncate(time.Millisecond))

	return &Result{
		Imported:       len(books),
		Collection:     collectionInfo,
		Duration:       time.Since(totalStart),
		ParseDuration:  parseDuration,
		ClearDuration:  clearDuration,
		InsertDuration: insertDuration,
	}, nil
}
