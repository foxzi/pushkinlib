-- Authors table
CREATE TABLE IF NOT EXISTS authors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

-- Genres table
CREATE TABLE IF NOT EXISTS genres (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

-- Series table
CREATE TABLE IF NOT EXISTS series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

-- Books table
CREATE TABLE IF NOT EXISTS books (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    series_id INTEGER,
    series_num INTEGER,
    genre_id INTEGER,
    year INTEGER,
    language TEXT,
    file_size INTEGER,
    archive_path TEXT,
    file_num TEXT,
    format TEXT,
    date_added DATETIME,
    rating INTEGER,
    annotation TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (series_id) REFERENCES series(id),
    FOREIGN KEY (genre_id) REFERENCES genres(id)
);

-- Book authors junction table (many-to-many)
CREATE TABLE IF NOT EXISTS book_authors (
    book_id TEXT,
    author_id INTEGER,
    PRIMARY KEY (book_id, author_id),
    FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE CASCADE
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_books_title ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_series ON books(series_id);
CREATE INDEX IF NOT EXISTS idx_books_genre ON books(genre_id);
CREATE INDEX IF NOT EXISTS idx_books_year ON books(year);
CREATE INDEX IF NOT EXISTS idx_books_language ON books(language);
CREATE INDEX IF NOT EXISTS idx_books_format ON books(format);
CREATE INDEX IF NOT EXISTS idx_books_date_added ON books(date_added);

CREATE INDEX IF NOT EXISTS idx_authors_name ON authors(name);
CREATE INDEX IF NOT EXISTS idx_genres_name ON genres(name);
CREATE INDEX IF NOT EXISTS idx_series_name ON series(name);

-- Full-text search will be implemented later when FTS5 is available