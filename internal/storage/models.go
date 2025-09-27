package storage

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Book represents a book in the database
type Book struct {
	ID          string    `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Authors     []Author  `json:"authors"`
	Series      *Series   `json:"series,omitempty"`
	SeriesNum   int       `json:"series_num,omitempty" db:"series_num"`
	Genre       *Genre    `json:"genre,omitempty"`
	Year        int       `json:"year,omitempty" db:"year"`
	Language    string    `json:"language" db:"language"`
	FileSize    int64     `json:"file_size" db:"file_size"`
	ArchivePath string    `json:"archive_path" db:"archive_path"`
	FileNum     string    `json:"file_num" db:"file_num"`
	Format      string    `json:"format" db:"format"`
	DateAdded   time.Time `json:"date_added" db:"date_added"`
	Rating      int       `json:"rating,omitempty" db:"rating"`
	Annotation  string    `json:"annotation,omitempty" db:"annotation"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Author represents an author
type Author struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// Series represents a book series
type Series struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// Genre represents a book genre
type Genre struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// BookFilter represents search and filter parameters
type BookFilter struct {
	Query     string   `json:"query,omitempty"`
	Authors   []string `json:"authors,omitempty"`
	Series    []string `json:"series,omitempty"`
	Genres    []string `json:"genres,omitempty"`
	Languages []string `json:"languages,omitempty"`
	Formats   []string `json:"formats,omitempty"`
	YearFrom  int      `json:"year_from,omitempty"`
	YearTo    int      `json:"year_to,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
	SortBy    string   `json:"sort_by,omitempty"` // title, year, date_added, relevance
	SortOrder string   `json:"sort_order,omitempty"` // asc, desc
}

// BookList represents paginated book results
type BookList struct {
	Books      []Book `json:"books"`
	Total      int    `json:"total"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	HasMore    bool   `json:"has_more"`
}

// StringArray is a helper type for JSON arrays in database
type StringArray []string

// Value implements driver.Valuer interface
func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return nil, nil
	}
	return json.Marshal(sa)
}

// Scan implements sql.Scanner interface
func (sa *StringArray) Scan(value interface{}) error {
	if value == nil {
		*sa = StringArray{}
		return nil
	}
	return json.Unmarshal(value.([]byte), sa)
}