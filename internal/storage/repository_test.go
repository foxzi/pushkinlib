package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/piligrim/pushkinlib/internal/inpx"
	"github.com/piligrim/pushkinlib/internal/storage"
)

func TestSearchBooksUsesFTS(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db)

	book := inpx.Book{
		ID:          "test-1",
		Title:       "Невероятные приключения",
		Authors:     []string{"Иван Иванов"},
		Series:      "Хроники",
		SeriesNum:   1,
		Genre:       "fantasy",
		Year:        2020,
		Language:    "ru",
		FileSize:    1234,
		ArchivePath: "books",
		FileNum:     "001",
		Format:      "fb2",
		Date:        time.Now(),
		Rating:      5,
		Annotation:  "Описание о путешествиях и открытиях.",
	}

	if err := repo.InsertBooks([]inpx.Book{book}); err != nil {
		t.Fatalf("failed to insert book: %v", err)
	}

	cases := []struct {
		name  string
		query string
	}{
		{name: "annotation", query: "путешеств"},
		{name: "author", query: "Иванов"},
		{name: "series", query: "Хроники"},
		{name: "title", query: "Невероятные"},
		{name: "field_author", query: `author:"Иван Иванов"`},
		{name: "field_title", query: `title:"Невероятные приключения"`},
		{name: "field_series", query: `series:"Хроники"`},
		{name: "field_description", query: "description:путешеств"},
		{name: "mixed_field_and_general", query: "author:Иванов приключения"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := repo.SearchBooks(storage.BookFilter{Query: tc.query})
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if result.Total != 1 {
				t.Fatalf("expected 1 result, got %d", result.Total)
			}
			if len(result.Books) != 1 {
				t.Fatalf("expected 1 book, got %d", len(result.Books))
			}
			if result.Books[0].ID != book.ID {
				t.Fatalf("unexpected book id: %s", result.Books[0].ID)
			}
		})
	}
}
