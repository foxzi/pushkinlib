package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/piligrim/pushkinlib/internal/inpx"
)

// Repository handles database operations for books
type Repository struct {
	db       *Database
	ftsFresh atomic.Bool
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

// ListAuthors returns a paginated list of authors
func (r *Repository) ListAuthors(limit, offset int) ([]Author, int, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.db.Query(
		"SELECT id, name FROM authors ORDER BY LOWER(name) LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query authors: %w", err)
	}
	defer rows.Close()

	var authors []Author
	for rows.Next() {
		var author Author
		if err := rows.Scan(&author.ID, &author.Name); err != nil {
			return nil, 0, fmt.Errorf("failed to scan author: %w", err)
		}
		authors = append(authors, author)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating authors: %w", err)
	}

	var total int
	if err := r.db.db.QueryRow("SELECT COUNT(*) FROM authors").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count authors: %w", err)
	}

	return authors, total, nil
}

// GetAuthorByID returns an author by ID
func (r *Repository) GetAuthorByID(authorID int) (*Author, error) {
	var author Author
	err := r.db.db.QueryRow("SELECT id, name FROM authors WHERE id = ?", authorID).Scan(&author.ID, &author.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load author %d: %w", authorID, err)
	}
	return &author, nil
}

// ListSeries returns a paginated list of series
func (r *Repository) ListSeries(limit, offset int) ([]Series, int, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.db.Query(
		"SELECT id, name FROM series ORDER BY LOWER(name) LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query series: %w", err)
	}
	defer rows.Close()

	var seriesList []Series
	for rows.Next() {
		var series Series
		if err := rows.Scan(&series.ID, &series.Name); err != nil {
			return nil, 0, fmt.Errorf("failed to scan series: %w", err)
		}
		seriesList = append(seriesList, series)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating series: %w", err)
	}

	var total int
	if err := r.db.db.QueryRow("SELECT COUNT(*) FROM series").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count series: %w", err)
	}

	return seriesList, total, nil
}

// GetSeriesByID returns a series by ID
func (r *Repository) GetSeriesByID(seriesID int) (*Series, error) {
	var series Series
	err := r.db.db.QueryRow("SELECT id, name FROM series WHERE id = ?", seriesID).Scan(&series.ID, &series.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load series %d: %w", seriesID, err)
	}
	return &series, nil
}

// ListGenres returns a paginated list of genres
func (r *Repository) ListGenres(limit, offset int) ([]Genre, int, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.db.Query(
		"SELECT id, name FROM genres ORDER BY LOWER(name) LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query genres: %w", err)
	}
	defer rows.Close()

	var genres []Genre
	for rows.Next() {
		var genre Genre
		if err := rows.Scan(&genre.ID, &genre.Name); err != nil {
			return nil, 0, fmt.Errorf("failed to scan genre: %w", err)
		}
		genres = append(genres, genre)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating genres: %w", err)
	}

	var total int
	if err := r.db.db.QueryRow("SELECT COUNT(*) FROM genres").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count genres: %w", err)
	}

	return genres, total, nil
}

// GetGenreByID returns a genre by ID
func (r *Repository) GetGenreByID(genreID int) (*Genre, error) {
	var genre Genre
	err := r.db.db.QueryRow("SELECT id, name FROM genres WHERE id = ?", genreID).Scan(&genre.ID, &genre.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load genre %d: %w", genreID, err)
	}
	return &genre, nil
}

// InsertBooks inserts multiple books from INPX parsing
func (r *Repository) InsertBooks(books []inpx.Book) error {
	if len(books) == 0 {
		return nil
	}

	var snapshot pragmaSnapshot
	if snap, err := r.captureBulkImportPragmaSnapshot(); err != nil {
		log.Printf("InsertBooks: failed to capture PRAGMA snapshot: %v", err)
	} else {
		snapshot = *snap

		if err := r.setPragmaInt("synchronous", 0); err != nil {
			log.Printf("InsertBooks: PRAGMA synchronous optimization skipped: %v", err)
		} else {
			defer func(value int) {
				if restoreErr := r.setPragmaInt("synchronous", value); restoreErr != nil {
					log.Printf("InsertBooks: failed to restore PRAGMA synchronous: %v", restoreErr)
				}
			}(snapshot.synchronous)
		}

		if err := r.setPragmaInt("temp_store", 2); err != nil {
			log.Printf("InsertBooks: PRAGMA temp_store optimization skipped: %v", err)
		} else {
			defer func(value int) {
				if restoreErr := r.setPragmaInt("temp_store", value); restoreErr != nil {
					log.Printf("InsertBooks: failed to restore PRAGMA temp_store: %v", restoreErr)
				}
			}(snapshot.tempStore)
		}

		if err := r.setPragmaInt("cache_size", -200000); err != nil {
			log.Printf("InsertBooks: PRAGMA cache_size optimization skipped: %v", err)
		} else {
			defer func(value int) {
				if restoreErr := r.setPragmaInt("cache_size", value); restoreErr != nil {
					log.Printf("InsertBooks: failed to restore PRAGMA cache_size: %v", restoreErr)
				}
			}(snapshot.cacheSize)
		}

		if snapshot.journalMode != "" {
			if newMode, err := r.setPragmaJournalMode("MEMORY"); err != nil {
				log.Printf("InsertBooks: PRAGMA journal_mode optimization skipped: %v", err)
			} else if !strings.EqualFold(newMode, "MEMORY") {
				log.Printf("InsertBooks: journal_mode remained %s, expected MEMORY", newMode)
			} else {
				defer func(mode string) {
					if _, restoreErr := r.setPragmaJournalMode(mode); restoreErr != nil {
						log.Printf("InsertBooks: failed to restore PRAGMA journal_mode=%s: %v", mode, restoreErr)
					}
				}(snapshot.journalMode)
			}
		}
	}

	tx, err := r.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	skipFTSDelete := r.ftsFresh.Swap(false)

	bookStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO books
		(id, title, series_id, series_num, genre_id, year, language,
		 file_size, archive_path, file_num, format, date_added, rating, annotation, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare book insert statement: %w", err)
	}
	defer bookStmt.Close()

	bookAuthorStmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO book_authors (book_id, author_id)
		VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare book author statement: %w", err)
	}
	defer bookAuthorStmt.Close()

	var ftsDeleteStmt *sql.Stmt
	if !skipFTSDelete {
		ftsDeleteStmt, err = tx.Prepare("DELETE FROM books_fts WHERE book_id = ?")
		if err != nil {
			return fmt.Errorf("failed to prepare books_fts delete statement: %w", err)
		}
		defer ftsDeleteStmt.Close()
	}

	ftsInsertStmt, err := tx.Prepare(`
		INSERT INTO books_fts (book_id, title, annotation, authors, series)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare books_fts insert statement: %w", err)
	}
	defer ftsInsertStmt.Close()

	authorCache := make(map[string]int, 1024)
	seriesCache := make(map[string]int, 256)
	genreCache := make(map[string]int, 128)

	for i, book := range books {
		if err := r.insertBookTx(tx, book, bookStmt, bookAuthorStmt, ftsDeleteStmt, ftsInsertStmt, authorCache, seriesCache, genreCache, skipFTSDelete); err != nil {
			return fmt.Errorf("failed to insert book %s: %w", book.ID, err)
		}

		if (i+1)%50000 == 0 || i+1 == len(books) {
			log.Printf("Reindex: inserted %d/%d books", i+1, len(books))
		}
	}

	return tx.Commit()
}

type pragmaSnapshot struct {
	synchronous int
	tempStore   int
	cacheSize   int
	journalMode string
}

func (r *Repository) captureBulkImportPragmaSnapshot() (*pragmaSnapshot, error) {
	synchronous, err := r.pragmaInt("synchronous")
	if err != nil {
		return nil, err
	}

	tempStore, err := r.pragmaInt("temp_store")
	if err != nil {
		return nil, err
	}

	cacheSize, err := r.pragmaInt("cache_size")
	if err != nil {
		return nil, err
	}

	journalMode, err := r.pragmaString("journal_mode")
	if err != nil {
		return nil, err
	}

	return &pragmaSnapshot{
		synchronous: synchronous,
		tempStore:   tempStore,
		cacheSize:   cacheSize,
		journalMode: journalMode,
	}, nil
}

func (r *Repository) pragmaInt(name string) (int, error) {
	var value int
	query := fmt.Sprintf("PRAGMA %s", name)
	if err := r.db.db.QueryRow(query).Scan(&value); err != nil {
		return 0, fmt.Errorf("failed to read PRAGMA %s: %w", name, err)
	}
	return value, nil
}

func (r *Repository) setPragmaInt(name string, value int) error {
	query := fmt.Sprintf("PRAGMA %s = %d", name, value)
	if _, err := r.db.db.Exec(query); err != nil {
		return fmt.Errorf("failed to set PRAGMA %s: %w", name, err)
	}
	return nil
}

func (r *Repository) pragmaString(name string) (string, error) {
	var value string
	query := fmt.Sprintf("PRAGMA %s", name)
	if err := r.db.db.QueryRow(query).Scan(&value); err != nil {
		return "", fmt.Errorf("failed to read PRAGMA %s: %w", name, err)
	}
	return value, nil
}

func (r *Repository) setPragmaJournalMode(mode string) (string, error) {
	normalized := strings.ToUpper(mode)
	query := fmt.Sprintf("PRAGMA journal_mode = %s", normalized)
	var result string
	if err := r.db.db.QueryRow(query).Scan(&result); err != nil {
		return "", fmt.Errorf("failed to set PRAGMA journal_mode=%s: %w", normalized, err)
	}
	return strings.ToUpper(result), nil
}

// insertBookTx inserts a single book within a transaction
func (r *Repository) insertBookTx(
	tx *sql.Tx,
	book inpx.Book,
	bookStmt, bookAuthorStmt, ftsDeleteStmt, ftsInsertStmt *sql.Stmt,
	authorCache, seriesCache, genreCache map[string]int,
	skipFTSDelete bool,
) error {
	var seriesID sql.NullInt64
	if book.Series != "" {
		id, err := r.getOrCreateSeriesTx(tx, book.Series, seriesCache)
		if err != nil {
			return err
		}
		seriesID = sql.NullInt64{Int64: int64(id), Valid: true}
	}

	var genreID sql.NullInt64
	if book.Genre != "" {
		id, err := r.getOrCreateGenreTx(tx, book.Genre, genreCache)
		if err != nil {
			return err
		}
		genreID = sql.NullInt64{Int64: int64(id), Valid: true}
	}

	if _, err := bookStmt.Exec(
		book.ID,
		book.Title,
		seriesID,
		book.SeriesNum,
		genreID,
		book.Year,
		book.Language,
		book.FileSize,
		book.ArchivePath,
		book.FileNum,
		book.Format,
		book.Date,
		book.Rating,
		book.Annotation,
		time.Now(),
	); err != nil {
		return err
	}

	for _, authorName := range book.Authors {
		if authorName == "" {
			continue
		}

		authorID, err := r.getOrCreateAuthorTx(tx, authorName, authorCache)
		if err != nil {
			return err
		}

		if _, err := bookAuthorStmt.Exec(book.ID, authorID); err != nil {
			return err
		}
	}

	if !skipFTSDelete && ftsDeleteStmt != nil {
		if _, err := ftsDeleteStmt.Exec(book.ID); err != nil {
			return err
		}
	}

	authorsText := strings.Join(book.Authors, " ")
	if _, err := ftsInsertStmt.Exec(book.ID, book.Title, book.Annotation, authorsText, book.Series); err != nil {
		return err
	}

	return nil
}

// getOrCreateAuthorTx gets or creates an author and returns its ID
func (r *Repository) getOrCreateAuthorTx(tx *sql.Tx, name string, cache map[string]int) (int, error) {
	if cache != nil {
		if id, ok := cache[name]; ok {
			return id, nil
		}
	}

	result, err := tx.Exec("INSERT INTO authors (name) VALUES (?)", name)
	if err == nil {
		lastID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}

		id := int(lastID)
		if cache != nil {
			cache[name] = id
		}
		return id, nil
	}

	if !isUniqueConstraintError(err) {
		return 0, err
	}

	var id int
	if err := tx.QueryRow("SELECT id FROM authors WHERE name = ?", name).Scan(&id); err != nil {
		return 0, err
	}

	if cache != nil {
		cache[name] = id
	}
	return id, nil
}

// getOrCreateSeriesTx gets or creates a series and returns its ID
func (r *Repository) getOrCreateSeriesTx(tx *sql.Tx, name string, cache map[string]int) (int, error) {
	if cache != nil {
		if id, ok := cache[name]; ok {
			return id, nil
		}
	}

	result, err := tx.Exec("INSERT INTO series (name) VALUES (?)", name)
	if err == nil {
		lastID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}

		id := int(lastID)
		if cache != nil {
			cache[name] = id
		}
		return id, nil
	}

	if !isUniqueConstraintError(err) {
		return 0, err
	}

	var id int
	if err := tx.QueryRow("SELECT id FROM series WHERE name = ?", name).Scan(&id); err != nil {
		return 0, err
	}

	if cache != nil {
		cache[name] = id
	}
	return id, nil
}

// getOrCreateGenreTx gets or creates a genre and returns its ID
func (r *Repository) getOrCreateGenreTx(tx *sql.Tx, name string, cache map[string]int) (int, error) {
	if cache != nil {
		if id, ok := cache[name]; ok {
			return id, nil
		}
	}

	result, err := tx.Exec("INSERT INTO genres (name) VALUES (?)", name)
	if err == nil {
		lastID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}

		id := int(lastID)
		if cache != nil {
			cache[name] = id
		}
		return id, nil
	}

	if !isUniqueConstraintError(err) {
		return 0, err
	}

	var id int
	if err := tx.QueryRow("SELECT id FROM genres WHERE name = ?", name).Scan(&id); err != nil {
		return 0, err
	}

	if cache != nil {
		cache[name] = id
	}
	return id, nil
}

func isUniqueConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code == sqlite3.ErrConstraint {
			return true
		}
		switch sqliteErr.ExtendedCode {
		case sqlite3.ErrConstraintUnique, sqlite3.ErrConstraintPrimaryKey:
			return true
		}
	}
	return false
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

	if err := tx.Commit(); err != nil {
		return err
	}

	r.ftsFresh.Store(true)
	return nil
}
