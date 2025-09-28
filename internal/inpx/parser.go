package inpx

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
)

// Parser handles INPX file parsing
type Parser struct{}

// NewParser creates a new INPX parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseINPX parses an INPX file and returns books and collection info
func (p *Parser) ParseINPX(inpxPath string) ([]Book, *CollectionInfo, error) {
	reader, err := zip.OpenReader(inpxPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open INPX file: %w", err)
	}
	defer reader.Close()

	var books []Book
	var collectionInfo *CollectionInfo

	for _, file := range reader.File {
		switch {
		case strings.HasSuffix(file.Name, ".inp"):
			inpBooks, err := p.parseINPFile(file)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse INP file %s: %w", file.Name, err)
			}
			books = append(books, inpBooks...)

		case file.Name == "collection.info":
			collectionInfo, err = p.parseCollectionInfo(file)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse collection.info: %w", err)
			}
		}
	}

	return books, collectionInfo, nil
}

// parseINPFile parses a single INP file
func (p *Parser) parseINPFile(file *zip.File) ([]Book, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var books []Book
	scanner := bufio.NewScanner(rc)
	defaultArchive := strings.TrimSuffix(path.Base(file.Name), ".inp")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		book, err := p.parseINPLine(line)
		if err != nil {
			// Log error but continue parsing other lines
			continue
		}

		if book.ArchivePath == "" || book.ArchivePath == book.ID {
			book.ArchivePath = defaultArchive
		}
		book.FileNum = book.ID

		books = append(books, book)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return books, nil
}

// parseINPLine parses a single line from INP file
// Format: AUTHOR\x04GENRE\x04TITLE\x04SERIES\x04SERIES_NUM\x04BOOK_ID\x04SIZE\x04ARCHIVE_PATH\x04FILE_NUM\x04FORMAT\x04DATE\x04LANG\x04RATING\x04ANNOTATION\x04
func (p *Parser) parseINPLine(line string) (Book, error) {
	parts := strings.Split(line, "\x04")
	if len(parts) < 13 {
		return Book{}, fmt.Errorf("invalid INP line format: %s", line)
	}

	// Parse authors (comma-separated)
	authors := p.parseAuthors(parts[0])

	// Parse series number
	seriesNum, _ := strconv.Atoi(parts[4])

	// Parse file size
	fileSize, _ := strconv.ParseInt(parts[6], 10, 64)

	// Parse year from date (YYYY-MM-DD format)
	year := p.parseYear(parts[10])

	// Parse date
	date := p.parseDate(parts[10])

	// Parse rating
	rating, _ := strconv.Atoi(parts[12])

	// Parse annotation if present
	var annotation string
	if len(parts) > 13 && parts[13] != "" {
		annotation = parts[13]
	}

	book := Book{
		ID:          parts[5],
		Title:       parts[2],
		Authors:     authors,
		Series:      parts[3],
		SeriesNum:   seriesNum,
		Genre:       parts[1],
		Year:        year,
		Language:    parts[11],
		FileSize:    fileSize,
		ArchivePath: parts[7],
		FileNum:     parts[8],
		Format:      parts[9],
		Date:        date,
		Rating:      rating,
		Annotation:  annotation,
	}

	return book, nil
}

// parseAuthors splits author string by comma and trims spaces
func (p *Parser) parseAuthors(authorStr string) []string {
	if authorStr == "" {
		return []string{}
	}

	// Remove trailing colon if present
	authorStr = strings.TrimSuffix(authorStr, ":")

	parts := strings.Split(authorStr, ",")
	authors := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			authors = append(authors, trimmed)
		}
	}

	return authors
}

// parseYear extracts year from date string
func (p *Parser) parseYear(dateStr string) int {
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}
	return 0
}

// parseDate parses date from YYYY-MM-DD format
func (p *Parser) parseDate(dateStr string) time.Time {
	if date, err := time.Parse("2006-01-02", dateStr); err == nil {
		return date
	}
	return time.Time{}
}

// parseCollectionInfo parses collection.info file
func (p *Parser) parseCollectionInfo(file *zip.File) (*CollectionInfo, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < 4 {
		return nil, fmt.Errorf("invalid collection.info format")
	}

	info := &CollectionInfo{
		Name:        strings.TrimSpace(lines[0]),
		Version:     strings.TrimSpace(lines[1]),
		Description: strings.TrimSpace(lines[3]),
	}

	// Extract date from name if present
	if strings.Contains(info.Name, " - ") {
		parts := strings.Split(info.Name, " - ")
		if len(parts) > 1 {
			info.Date = strings.TrimSpace(parts[len(parts)-1])
		}
	}

	return info, nil
}
