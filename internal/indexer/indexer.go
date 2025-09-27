package indexer

import (
	"errors"
	"fmt"
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
	Imported   int
	Collection *inpx.CollectionInfo
	Duration   time.Duration
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
	start := time.Now()

	books, collectionInfo, err := parser.ParseINPX(inpxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inpx: %w", err)
	}

	if err := repo.ClearAllBooks(); err != nil {
		return nil, fmt.Errorf("failed to clear existing data: %w", err)
	}

	if err := repo.InsertBooks(books); err != nil {
		return nil, fmt.Errorf("failed to insert books: %w", err)
	}

	return &Result{
		Imported:   len(books),
		Collection: collectionInfo,
		Duration:   time.Since(start),
	}, nil
}
