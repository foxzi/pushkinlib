package inpx

import "time"

// Book represents a book entry from INPX
type Book struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Authors     []string  `json:"authors"`
	Series      string    `json:"series,omitempty"`
	SeriesNum   int       `json:"series_num,omitempty"`
	Genre       string    `json:"genre"`
	Year        int       `json:"year,omitempty"`
	Language    string    `json:"language"`
	FileSize    int64     `json:"file_size"`
	ArchivePath string    `json:"archive_path"`
	FileNum     string    `json:"file_num"`
	Format      string    `json:"format"`
	Date        time.Time `json:"date"`
	Rating      int       `json:"rating,omitempty"`
	Annotation  string    `json:"annotation,omitempty"`
}

// CollectionInfo represents metadata about the collection
type CollectionInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Date        string `json:"date"`
}