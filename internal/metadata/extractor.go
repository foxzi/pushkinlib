package metadata

import (
	"archive/zip"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Extractor handles metadata extraction from book files
type Extractor struct{}

// NewExtractor creates a new metadata extractor
func NewExtractor() *Extractor {
	return &Extractor{}
}

// ExtractFromFile extracts metadata from a book file
func (e *Extractor) ExtractFromFile(filePath string) (*BookMetadata, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := filepath.Base(filePath)

	metadata := &BookMetadata{
		FilePath: filePath,
		FileName: fileName,
		FileSize: fileInfo.Size(),
		Date:     fileInfo.ModTime(),
	}

	// Generate unique ID from file path and size
	metadata.ID = e.generateID(filePath, fileInfo.Size())

	switch ext {
	case ".fb2":
		metadata.Format = "fb2"
		return e.extractFB2Metadata(metadata)
	case ".zip":
		// Check if it's FB2 zip
		if e.isFB2Zip(filePath) {
			metadata.Format = "fb2"
			return e.extractFB2ZipMetadata(metadata)
		}
		return nil, fmt.Errorf("unsupported zip format")
	case ".epub":
		metadata.Format = "epub"
		return e.extractEPUBMetadata(metadata)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// generateID generates unique ID for book
func (e *Extractor) generateID(filePath string, size int64) string {
	data := fmt.Sprintf("%s:%d", filePath, size)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)[:12]
}

// extractFB2Metadata extracts metadata from FB2 file
func (e *Extractor) extractFB2Metadata(metadata *BookMetadata) (*BookMetadata, error) {
	file, err := os.Open(metadata.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return e.parseFB2Content(file, metadata)
}

// extractFB2ZipMetadata extracts metadata from FB2 zip file
func (e *Extractor) extractFB2ZipMetadata(metadata *BookMetadata) (*BookMetadata, error) {
	zipReader, err := zip.OpenReader(metadata.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer zipReader.Close()

	// Find FB2 file in zip
	for _, file := range zipReader.File {
		if strings.HasSuffix(strings.ToLower(file.Name), ".fb2") {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			return e.parseFB2Content(rc, metadata)
		}
	}

	return nil, fmt.Errorf("no FB2 file found in zip")
}

// parseFB2Content parses FB2 content from reader
func (e *Extractor) parseFB2Content(reader io.Reader, metadata *BookMetadata) (*BookMetadata, error) {
	decoder := xml.NewDecoder(reader)

	// Find description element
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse XML: %w", err)
		}

		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "description" {
			var desc FB2Description
			if err := decoder.DecodeElement(&desc, &start); err != nil {
				return nil, fmt.Errorf("failed to decode description: %w", err)
			}

			return e.fillMetadataFromFB2(metadata, &desc), nil
		}
	}

	return nil, fmt.Errorf("no description found in FB2")
}

// fillMetadataFromFB2 fills metadata from FB2 description
func (e *Extractor) fillMetadataFromFB2(metadata *BookMetadata, desc *FB2Description) *BookMetadata {
	titleInfo := &desc.TitleInfo

	// Title
	metadata.Title = strings.TrimSpace(titleInfo.BookTitle)

	// Authors
	for _, author := range titleInfo.Authors {
		authorName := e.formatAuthorName(author)
		if authorName != "" {
			metadata.Authors = append(metadata.Authors, authorName)
		}
	}

	// Genres
	for _, genre := range titleInfo.Genres {
		if genre.Value != "" {
			metadata.Genres = append(metadata.Genres, strings.TrimSpace(genre.Value))
		}
	}

	// Language
	metadata.Language = strings.TrimSpace(titleInfo.Lang)
	if metadata.Language == "" {
		metadata.Language = "ru" // Default to Russian
	}

	// Series
	if titleInfo.Sequence != nil {
		metadata.Series = strings.TrimSpace(titleInfo.Sequence.Name)
		if titleInfo.Sequence.Number != "" {
			if num, err := strconv.Atoi(titleInfo.Sequence.Number); err == nil {
				metadata.SeriesNum = num
			}
		}
	}

	// Annotation
	if titleInfo.Annotation != nil {
		metadata.Annotation = e.cleanAnnotation(titleInfo.Annotation.Content)
	}

	// Keywords
	if titleInfo.Keywords != "" {
		keywords := strings.Split(titleInfo.Keywords, ",")
		for _, keyword := range keywords {
			if trimmed := strings.TrimSpace(keyword); trimmed != "" {
				metadata.Keywords = append(metadata.Keywords, trimmed)
			}
		}
	}

	// Date/Year
	if titleInfo.Date != nil {
		if titleInfo.Date.Value != "" {
			metadata.Year = e.extractYear(titleInfo.Date.Value)
		} else if titleInfo.Date.Text != "" {
			metadata.Year = e.extractYear(titleInfo.Date.Text)
		}
	}

	// If no year from title-info, try publish-info
	if metadata.Year == 0 && desc.PublishInfo != nil && desc.PublishInfo.Year != "" {
		metadata.Year = e.extractYear(desc.PublishInfo.Year)
	}

	return metadata
}

// formatAuthorName formats author name from FB2 author struct
func (e *Extractor) formatAuthorName(author FB2Author) string {
	var parts []string

	if author.LastName != "" {
		parts = append(parts, author.LastName)
	}
	if author.FirstName != "" {
		parts = append(parts, author.FirstName)
	}
	if author.MiddleName != "" {
		parts = append(parts, author.MiddleName)
	}

	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}

	if author.Nickname != "" {
		return author.Nickname
	}

	return ""
}

// cleanAnnotation cleans annotation text
func (e *Extractor) cleanAnnotation(content string) string {
	// Remove XML tags
	content = strings.ReplaceAll(content, "<p>", "")
	content = strings.ReplaceAll(content, "</p>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br>", "\n")

	// Clean up extra whitespace
	lines := strings.Split(content, "\n")
	var cleanLines []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	result := strings.Join(cleanLines, "\n")
	if len(result) > 1000 {
		result = result[:1000] + "..."
	}

	return result
}

// extractYear extracts year from date string
func (e *Extractor) extractYear(dateStr string) int {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 1000 && year <= time.Now().Year() {
			return year
		}
	}
	return 0
}

// isFB2Zip checks if zip file contains FB2
func (e *Extractor) isFB2Zip(filePath string) bool {
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return false
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if strings.HasSuffix(strings.ToLower(file.Name), ".fb2") {
			return true
		}
	}

	return false
}

// extractEPUBMetadata extracts metadata from EPUB file (basic implementation)
func (e *Extractor) extractEPUBMetadata(metadata *BookMetadata) (*BookMetadata, error) {
	// Basic EPUB support - extract from filename for now
	name := strings.TrimSuffix(metadata.FileName, filepath.Ext(metadata.FileName))

	metadata.Title = name
	metadata.Language = "en"
	metadata.Genres = []string{"unknown"}

	// Try to extract author from filename patterns like "Author - Title.epub"
	if parts := strings.Split(name, " - "); len(parts) >= 2 {
		metadata.Authors = []string{strings.TrimSpace(parts[0])}
		metadata.Title = strings.TrimSpace(parts[1])
	}

	return metadata, nil
}