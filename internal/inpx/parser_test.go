package inpx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseINPX(t *testing.T) {
	// Use the sample data
	inpxPath := filepath.Join("..", "..", "sample-data", "flibusta_fb2_local.inpx")

	if _, err := os.Stat(inpxPath); err != nil {
		if os.IsNotExist(err) {
			t.Skip("sample INPX file not found; skipping integration test")
		}
		t.Fatalf("failed to access INPX file: %v", err)
	}

	parser := NewParser()
	books, collectionInfo, err := parser.ParseINPX(inpxPath)

	if err != nil {
		t.Fatalf("Failed to parse INPX: %v", err)
	}

	if len(books) == 0 {
		t.Fatal("No books found")
	}

	t.Logf("Parsed %d books", len(books))

	// Test collection info
	if collectionInfo == nil {
		t.Fatal("Collection info not found")
	}

	t.Logf("Collection: %s", collectionInfo.Name)
	t.Logf("Description: %s", collectionInfo.Description)

	// Test first book
	firstBook := books[0]
	if firstBook.ID == "" {
		t.Error("Book ID is empty")
	}

	if firstBook.Title == "" {
		t.Error("Book title is empty")
	}

	if len(firstBook.Authors) == 0 {
		t.Error("Book has no authors")
	}

	t.Logf("First book: %s by %v", firstBook.Title, firstBook.Authors)
}
