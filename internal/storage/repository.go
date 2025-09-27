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

const bookSelectColumns = `
	b.id, b.title, b.series_id, b.series_num, b.genre_id, b.year,
	b.language, b.file_size, b.archive_path, b.file_num, b.format,
	b.date_added, b.rating, b.annotation, b.created_at, b.updated_at,
	s.name as series_name, g.name as genre_name`

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

	// Update full-text search index
	if _, err := tx.Exec("DELETE FROM books_fts WHERE book_id = ?", book.ID); err != nil {
		return err
	}

	authorsText := strings.Join(book.Authors, " ")
	if _, err := tx.Exec(
		"INSERT INTO books_fts (book_id, title, annotation, authors, series) VALUES (?, ?, ?, ?, ?)",
		book.ID, book.Title, book.Annotation, authorsText, book.Series,
	); err != nil {
		return err
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
	sanitized := filter
	if sanitized.Limit <= 0 {
		sanitized.Limit = 30
	}
	if sanitized.Offset < 0 {
		sanitized.Offset = 0
	}

	query, queryArgs, countQuery, countArgs := r.buildSearchSQL(sanitized)

	var total int
	if err := r.db.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count books: %w", err)
	}

	rows, err := r.db.db.Query(query, queryArgs...)
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

	return &BookList{
		Books:   books,
		Total:   total,
		Limit:   sanitized.Limit,
		Offset:  sanitized.Offset,
		HasMore: sanitized.Offset+sanitized.Limit < total,
	}, nil
}

func (r *Repository) buildSearchSQL(filter BookFilter) (string, []interface{}, string, []interface{}) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 30
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	joins := []string{
		"LEFT JOIN series s ON b.series_id = s.id",
		"LEFT JOIN genres g ON b.genre_id = g.id",
	}
	conditions := make([]string, 0)
	baseArgs := make([]interface{}, 0)
	joinedAuthors := false
	hasFTS := false

	addAuthorJoin := func() {
		if !joinedAuthors {
			joins = append(joins, "LEFT JOIN book_authors ba ON b.id = ba.book_id")
			joins = append(joins, "LEFT JOIN authors a ON ba.author_id = a.id")
			joinedAuthors = true
		}
	}

	if strings.TrimSpace(filter.Query) != "" {
		ftsQuery, fallback := prepareFTSSearch(filter.Query)
		if ftsQuery != "" {
			hasFTS = true
			joins = append(joins, "JOIN books_fts ON books_fts.book_id = b.id")
			conditions = append(conditions, "books_fts MATCH ?")
			baseArgs = append(baseArgs, ftsQuery)
		} else if fallback != "" {
			addAuthorJoin()
			like := "%" + strings.ToLower(fallback) + "%"
			conditions = append(conditions, "(LOWER(b.title) LIKE ? OR LOWER(b.annotation) LIKE ? OR LOWER(a.name) LIKE ? OR LOWER(s.name) LIKE ?)")
			baseArgs = append(baseArgs, like, like, like, like)
		}
	}

	if len(filter.Authors) > 0 {
		addAuthorJoin()
		placeholders := createPlaceholders(len(filter.Authors))
		conditions = append(conditions, fmt.Sprintf("a.name IN (%s)", placeholders))
		for _, author := range filter.Authors {
			baseArgs = append(baseArgs, author)
		}
	}

	if len(filter.Series) > 0 {
		placeholders := createPlaceholders(len(filter.Series))
		conditions = append(conditions, fmt.Sprintf("s.name IN (%s)", placeholders))
		for _, series := range filter.Series {
			baseArgs = append(baseArgs, series)
		}
	}

	if len(filter.Genres) > 0 {
		placeholders := createPlaceholders(len(filter.Genres))
		conditions = append(conditions, fmt.Sprintf("g.name IN (%s)", placeholders))
		for _, genre := range filter.Genres {
			baseArgs = append(baseArgs, genre)
		}
	}

	if len(filter.Languages) > 0 {
		placeholders := createPlaceholders(len(filter.Languages))
		conditions = append(conditions, fmt.Sprintf("b.language IN (%s)", placeholders))
		for _, language := range filter.Languages {
			baseArgs = append(baseArgs, language)
		}
	}

	if len(filter.Formats) > 0 {
		placeholders := createPlaceholders(len(filter.Formats))
		conditions = append(conditions, fmt.Sprintf("b.format IN (%s)", placeholders))
		for _, format := range filter.Formats {
			baseArgs = append(baseArgs, format)
		}
	}

	if filter.YearFrom > 0 {
		conditions = append(conditions, "b.year >= ?")
		baseArgs = append(baseArgs, filter.YearFrom)
	}

	if filter.YearTo > 0 {
		conditions = append(conditions, "b.year <= ?")
		baseArgs = append(baseArgs, filter.YearTo)
	}

	orderClause := buildOrderClause(filter.SortBy, filter.SortOrder, hasFTS)

	var queryBuilder strings.Builder
	queryBuilder.WriteString("SELECT ")
	queryBuilder.WriteString(bookSelectColumns)
	queryBuilder.WriteString(" FROM books b")
	for _, join := range joins {
		queryBuilder.WriteString(" ")
		queryBuilder.WriteString(join)
	}
	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}
	if joinedAuthors {
		queryBuilder.WriteString(" GROUP BY b.id")
	}
	queryBuilder.WriteString(orderClause)
	queryBuilder.WriteString(" LIMIT ? OFFSET ?")

	queryArgs := make([]interface{}, 0, len(baseArgs)+2)
	queryArgs = append(queryArgs, baseArgs...)
	queryArgs = append(queryArgs, limit, offset)

	var countBuilder strings.Builder
	countBuilder.WriteString("SELECT COUNT(DISTINCT b.id) FROM books b")
	for _, join := range joins {
		countBuilder.WriteString(" ")
		countBuilder.WriteString(join)
	}
	if len(conditions) > 0 {
		countBuilder.WriteString(" WHERE ")
		countBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	countArgs := make([]interface{}, 0, len(baseArgs))
	countArgs = append(countArgs, baseArgs...)

	return queryBuilder.String(), queryArgs, countBuilder.String(), countArgs
}

func buildOrderClause(sortBy, sortOrder string, hasFTS bool) string {
	if sortBy == "" && hasFTS {
		sortBy = "relevance"
	}

	var column string
	switch sortBy {
	case "year":
		column = "b.year"
	case "date_added":
		column = "b.date_added"
	case "relevance":
		if hasFTS {
			column = "bm25(books_fts)"
		} else {
			column = "b.title"
		}
	default:
		column = "b.title"
	}

	direction := "ASC"
	if strings.ToLower(sortOrder) == "desc" {
		direction = "DESC"
	}

	return " ORDER BY " + column + " " + direction
}

func createPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
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
	query := fmt.Sprintf(`SELECT %s FROM books b
		LEFT JOIN series s ON b.series_id = s.id
		LEFT JOIN genres g ON b.genre_id = g.id
		WHERE b.id = ?
		LIMIT 1`, bookSelectColumns)

	row := r.db.db.QueryRow(query, id)

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

	_, err = tx.Exec("DELETE FROM books_fts")
	if err != nil {
		return err
	}

	return tx.Commit()
}
