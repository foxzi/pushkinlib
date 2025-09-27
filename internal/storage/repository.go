package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/piligrim/pushkinlib/internal/inpx"
)

// Repository handles database operations for books
type Repository struct {
	db *Database
}

// NewRepository creates a new repository
func NewRepository(db *Database) *Repository {
	return &Repository{db: db}
}

// InsertBooks inserts multiple books from INPX parsing
func (r *Repository) InsertBooks(books []inpx.Book) error {
	tx, err := r.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, book := range books {
		if err := r.insertBookTx(tx, book); err != nil {
			return fmt.Errorf("failed to insert book %s: %w", book.ID, err)
		}
	}

	return tx.Commit()
}

// insertBookTx inserts a single book within a transaction
func (r *Repository) insertBookTx(tx *sql.Tx, book inpx.Book) error {
	// Insert or get series
	var seriesID *int
	if book.Series != "" {
		id, err := r.getOrCreateSeriesTx(tx, book.Series)
		if err != nil {
			return err
		}
		seriesID = &id
	}

	// Insert or get genre
	var genreID *int
	if book.Genre != "" {
		id, err := r.getOrCreateGenreTx(tx, book.Genre)
		if err != nil {
			return err
		}
		genreID = &id
	}

	// Insert book
	_, err := tx.Exec(`
		INSERT OR REPLACE INTO books
		(id, title, series_id, series_num, genre_id, year, language,
		 file_size, archive_path, file_num, format, date_added, rating, annotation, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.ID, book.Title, seriesID, book.SeriesNum, genreID, book.Year,
		book.Language, book.FileSize, book.ArchivePath, book.FileNum,
		book.Format, book.Date, book.Rating, book.Annotation, time.Now())
	if err != nil {
		return err
	}

	// Insert book authors
	for _, authorName := range book.Authors {
		if authorName == "" {
			continue
		}
		authorID, err := r.getOrCreateAuthorTx(tx, authorName)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO book_authors (book_id, author_id)
			VALUES (?, ?)`, book.ID, authorID)
		if err != nil {
			return err
		}
	}

	return nil
}

// getOrCreateAuthorTx gets or creates an author and returns its ID
func (r *Repository) getOrCreateAuthorTx(tx *sql.Tx, name string) (int, error) {
	var id int
	err := tx.QueryRow("SELECT id FROM authors WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := tx.Exec("INSERT INTO authors (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(lastID), nil
}

// getOrCreateSeriesTx gets or creates a series and returns its ID
func (r *Repository) getOrCreateSeriesTx(tx *sql.Tx, name string) (int, error) {
	var id int
	err := tx.QueryRow("SELECT id FROM series WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := tx.Exec("INSERT INTO series (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(lastID), nil
}

// getOrCreateGenreTx gets or creates a genre and returns its ID
func (r *Repository) getOrCreateGenreTx(tx *sql.Tx, name string) (int, error) {
	var id int
	err := tx.QueryRow("SELECT id FROM genres WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := tx.Exec("INSERT INTO genres (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(lastID), nil
}

// SearchBooks searches books with filters
func (r *Repository) SearchBooks(filter BookFilter) (*BookList, error) {
	if filter.Limit <= 0 {
		filter.Limit = 30
	}

	// Build query
	query := r.buildSearchQuery(filter)
	args := r.buildSearchArgs(filter)

	// Simple total count for now
	var total int
	err := r.db.db.QueryRow("SELECT COUNT(*) FROM books").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get books
	rows, err := r.db.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		book, err := r.scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan book: %w", err)
		}

		// Load authors for this book
		authors, err := r.getBookAuthors(book.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load authors for book %s: %w", book.ID, err)
		}
		book.Authors = authors

		books = append(books, book)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	hasMore := filter.Offset+filter.Limit < total

	return &BookList{
		Books:   books,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: hasMore,
	}, nil
}

// buildSearchQuery builds the SQL query for book search
func (r *Repository) buildSearchQuery(filter BookFilter) string {
	query := `
		SELECT b.id, b.title, b.series_id, b.series_num, b.genre_id, b.year,
		       b.language, b.file_size, b.archive_path, b.file_num, b.format,
		       b.date_added, b.rating, b.annotation, b.created_at, b.updated_at,
		       s.name as series_name, g.name as genre_name
		FROM books b
		LEFT JOIN series s ON b.series_id = s.id
		LEFT JOIN genres g ON b.genre_id = g.id`

	var conditions []string
	var needsAuthorJoin bool

	if filter.Query != "" {
		// Search in title, annotation, and author names
		needsAuthorJoin = true
		conditions = append(conditions, "(b.title LIKE ? OR b.annotation LIKE ? OR a.name LIKE ?)")
	}

	if len(filter.Authors) > 0 {
		needsAuthorJoin = true
		placeholders := strings.Repeat("?,", len(filter.Authors))
		placeholders = placeholders[:len(placeholders)-1] // Remove last comma
		conditions = append(conditions, fmt.Sprintf("a.name IN (%s)", placeholders))
	}

	// Add author join if needed
	if needsAuthorJoin {
		query += " LEFT JOIN book_authors ba ON b.id = ba.book_id LEFT JOIN authors a ON ba.author_id = a.id"
	}

	if len(filter.Series) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Series))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("s.name IN (%s)", placeholders))
	}

	if len(filter.Genres) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Genres))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("g.name IN (%s)", placeholders))
	}

	if len(filter.Languages) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Languages))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("b.language IN (%s)", placeholders))
	}

	if len(filter.Formats) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Formats))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("b.format IN (%s)", placeholders))
	}

	if filter.YearFrom > 0 {
		conditions = append(conditions, "b.year >= ?")
	}

	if filter.YearTo > 0 {
		conditions = append(conditions, "b.year <= ?")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Group by book ID to avoid duplicates when joining with authors
	if needsAuthorJoin {
		query += " GROUP BY b.id"
	}

	// Add sorting
	switch filter.SortBy {
	case "year":
		query += " ORDER BY b.year"
	case "date_added":
		query += " ORDER BY b.date_added"
	case "relevance":
		query += " ORDER BY b.title" // Simple title sort for now
	default:
		query += " ORDER BY b.title"
	}

	if filter.SortOrder == "desc" {
		query += " DESC"
	} else {
		query += " ASC"
	}

	query += " LIMIT ? OFFSET ?"

	return query
}

// buildSearchArgs builds the arguments for the search query
func (r *Repository) buildSearchArgs(filter BookFilter) []interface{} {
	var args []interface{}

	if filter.Query != "" {
		// Add wildcards for LIKE search (title, annotation, author)
		searchQuery := "%" + filter.Query + "%"
		args = append(args, searchQuery, searchQuery, searchQuery)
	}

	for _, author := range filter.Authors {
		args = append(args, author)
	}

	for _, series := range filter.Series {
		args = append(args, series)
	}

	for _, genre := range filter.Genres {
		args = append(args, genre)
	}

	for _, lang := range filter.Languages {
		args = append(args, lang)
	}

	for _, format := range filter.Formats {
		args = append(args, format)
	}

	if filter.YearFrom > 0 {
		args = append(args, filter.YearFrom)
	}

	if filter.YearTo > 0 {
		args = append(args, filter.YearTo)
	}

	args = append(args, filter.Limit, filter.Offset)

	return args
}

// scanBook scans a book from database row
func (r *Repository) scanBook(rows *sql.Rows) (Book, error) {
	var book Book
	var seriesID, genreID sql.NullInt64
	var seriesName, genreName sql.NullString

	err := rows.Scan(
		&book.ID, &book.Title, &seriesID, &book.SeriesNum, &genreID,
		&book.Year, &book.Language, &book.FileSize, &book.ArchivePath,
		&book.FileNum, &book.Format, &book.DateAdded, &book.Rating,
		&book.Annotation, &book.CreatedAt, &book.UpdatedAt,
		&seriesName, &genreName,
	)
	if err != nil {
		return book, err
	}

	if seriesID.Valid && seriesName.Valid {
		book.Series = &Series{
			ID:   int(seriesID.Int64),
			Name: seriesName.String,
		}
	}

	if genreID.Valid && genreName.Valid {
		book.Genre = &Genre{
			ID:   int(genreID.Int64),
			Name: genreName.String,
		}
	}

	return book, nil
}

// getBookAuthors gets all authors for a book
func (r *Repository) getBookAuthors(bookID string) ([]Author, error) {
	rows, err := r.db.db.Query(`
		SELECT a.id, a.name
		FROM authors a
		JOIN book_authors ba ON a.id = ba.author_id
		WHERE ba.book_id = ?
		ORDER BY a.name`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []Author
	for rows.Next() {
		var author Author
		if err := rows.Scan(&author.ID, &author.Name); err != nil {
			return nil, err
		}
		authors = append(authors, author)
	}

	return authors, rows.Err()
}

// GetBookByID gets a single book by ID
func (r *Repository) GetBookByID(id string) (*Book, error) {
	filter := BookFilter{
		Limit:  1,
		Offset: 0,
	}

	// Add WHERE condition for specific book ID
	query := r.buildSearchQuery(filter)
	query = strings.Replace(query, "ORDER BY", "WHERE b.id = ? ORDER BY", 1)

	row := r.db.db.QueryRow(query, id, 1, 0)

	book, err := r.scanBookRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get book: %w", err)
	}

	// Load authors
	authors, err := r.getBookAuthors(book.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load authors: %w", err)
	}
	book.Authors = authors

	return &book, nil
}

// scanBookRow scans a book from a single row
func (r *Repository) scanBookRow(row *sql.Row) (Book, error) {
	var book Book
	var seriesID, genreID sql.NullInt64
	var seriesName, genreName sql.NullString

	err := row.Scan(
		&book.ID, &book.Title, &seriesID, &book.SeriesNum, &genreID,
		&book.Year, &book.Language, &book.FileSize, &book.ArchivePath,
		&book.FileNum, &book.Format, &book.DateAdded, &book.Rating,
		&book.Annotation, &book.CreatedAt, &book.UpdatedAt,
		&seriesName, &genreName,
	)
	if err != nil {
		return book, err
	}

	if seriesID.Valid && seriesName.Valid {
		book.Series = &Series{
			ID:   int(seriesID.Int64),
			Name: seriesName.String,
		}
	}

	if genreID.Valid && genreName.Valid {
		book.Genre = &Genre{
			ID:   int(genreID.Int64),
			Name: genreName.String,
		}
	}

	return book, nil
}

// ClearAllBooks removes all books and related data
func (r *Repository) ClearAllBooks() error {
	tx, err := r.db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear in proper order due to foreign keys
	_, err = tx.Exec("DELETE FROM book_authors")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM books")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM authors")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM series")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM genres")
	if err != nil {
		return err
	}

	return tx.Commit()
}