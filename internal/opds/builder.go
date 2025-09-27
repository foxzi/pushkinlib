package opds

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/piligrim/pushkinlib/internal/storage"
)

// Builder creates OPDS feeds
type Builder struct {
	baseURL     string
	catalogTitle string
}

// NewBuilder creates a new OPDS builder
func NewBuilder(baseURL, catalogTitle string) *Builder {
	return &Builder{
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		catalogTitle: catalogTitle,
	}
}

// BuildRootFeed creates the root OPDS catalog
func (b *Builder) BuildRootFeed() *Feed {
	now := time.Now()

	feed := &Feed{
		Xmlns:     "http://www.w3.org/2005/Atom",
		XmlnsDC:   "http://purl.org/dc/terms/",
		XmlnsOPDS: "http://opds-spec.org/2010/catalog",

		ID:      b.baseURL + "/opds",
		Title:   b.catalogTitle,
		Updated: now,
		Icon:    b.baseURL + "/favicon.ico",

		Author: &Person{
			Name: b.catalogTitle,
			URI:  b.baseURL,
		},

		Links: []Link{
			{
				Rel:  "self",
				Type: TypeNavigation,
				Href: b.baseURL + "/opds",
			},
			{
				Rel:  RelStart,
				Type: TypeNavigation,
				Href: b.baseURL + "/opds",
			},
			{
				Rel:  RelSearch,
				Type: TypeSearch,
				Href: b.baseURL + "/opds/search?q={searchTerms}",
			},
		},

		Entries: []Entry{
			{
				ID:      b.baseURL + "/opds/books/new",
				Title:   "Новые поступления",
				Updated: now,
				Summary: "Недавно добавленные книги",
				Links: []Link{
					{
						Rel:  RelSubsection,
						Type: TypeAcquisition,
						Href: b.baseURL + "/opds/books/new",
					},
				},
			},
			{
				ID:      b.baseURL + "/opds/authors",
				Title:   "По авторам",
				Updated: now,
				Summary: "Каталог по авторам",
				Links: []Link{
					{
						Rel:  RelSubsection,
						Type: TypeNavigation,
						Href: b.baseURL + "/opds/authors",
					},
				},
			},
			{
				ID:      b.baseURL + "/opds/series",
				Title:   "По сериям",
				Updated: now,
				Summary: "Каталог по сериям",
				Links: []Link{
					{
						Rel:  RelSubsection,
						Type: TypeNavigation,
						Href: b.baseURL + "/opds/series",
					},
				},
			},
			{
				ID:      b.baseURL + "/opds/genres",
				Title:   "По жанрам",
				Updated: now,
				Summary: "Каталог по жанрам",
				Links: []Link{
					{
						Rel:  RelSubsection,
						Type: TypeNavigation,
						Href: b.baseURL + "/opds/genres",
					},
				},
			},
		},
	}

	return feed
}

// BuildBooksFeed creates a feed of books
func (b *Builder) BuildBooksFeed(books []storage.Book, title, feedID string, page, totalBooks int) *Feed {
	now := time.Now()
	pageSize := len(books)

	feed := &Feed{
		Xmlns:     "http://www.w3.org/2005/Atom",
		XmlnsDC:   "http://purl.org/dc/terms/",
		XmlnsOPDS: "http://opds-spec.org/2010/catalog",

		ID:      feedID,
		Title:   title,
		Updated: now,

		Author: &Person{
			Name: b.catalogTitle,
			URI:  b.baseURL,
		},

		Links: []Link{
			{
				Rel:  "self",
				Type: TypeAcquisition,
				Href: feedID,
			},
			{
				Rel:  RelStart,
				Type: TypeNavigation,
				Href: b.baseURL + "/opds",
			},
			{
				Rel:  RelUp,
				Type: TypeNavigation,
				Href: b.baseURL + "/opds",
			},
		},
	}

	// Add pagination links if needed
	if page > 1 {
		prevURL := b.buildPageURL(feedID, page-1)
		feed.Links = append(feed.Links, Link{
			Rel:  RelPrev,
			Type: TypeAcquisition,
			Href: prevURL,
		})
	}

	if page*pageSize < totalBooks {
		nextURL := b.buildPageURL(feedID, page+1)
		feed.Links = append(feed.Links, Link{
			Rel:  RelNext,
			Type: TypeAcquisition,
			Href: nextURL,
		})
	}

	// Convert books to entries
	for _, book := range books {
		entry := b.bookToEntry(book)
		feed.Entries = append(feed.Entries, entry)
	}

	return feed
}

// bookToEntry converts a storage.Book to OPDS Entry
func (b *Builder) bookToEntry(book storage.Book) Entry {
	entry := Entry{
		ID:      b.baseURL + "/opds/books/" + book.ID,
		Title:   book.Title,
		Updated: book.UpdatedAt,
		Summary: book.Annotation,
	}

	// Add authors
	for _, author := range book.Authors {
		entry.Authors = append(entry.Authors, Person{
			Name: author.Name,
		})
	}

	// Add genre
	if book.Genre != nil {
		entry.Categories = append(entry.Categories, Category{
			Term:  book.Genre.Name,
			Label: book.Genre.Name,
		})
	}

	// Add language
	if book.Language != "" {
		entry.Language = book.Language
	}

	// Add year
	if book.Year > 0 {
		entry.Issued = strconv.Itoa(book.Year)
	}

	// Add acquisition link
	downloadURL := b.baseURL + "/download/" + book.ID
	fileType := b.getFileType(book.Format)

	entry.Links = append(entry.Links, Link{
		Rel:    RelAcquisitionOpen,
		Type:   fileType,
		Href:   downloadURL,
		Length: book.FileSize,
	})

	// Add content with details
	if book.Series != nil || book.Year > 0 || book.Format != "" {
		var details []string

		if book.Series != nil {
			seriesInfo := book.Series.Name
			if book.SeriesNum > 0 {
				seriesInfo += fmt.Sprintf(" #%d", book.SeriesNum)
			}
			details = append(details, "Серия: "+seriesInfo)
		}

		if book.Year > 0 {
			details = append(details, "Год: "+strconv.Itoa(book.Year))
		}

		if book.Format != "" {
			details = append(details, "Формат: "+strings.ToUpper(book.Format))
		}

		if book.FileSize > 0 {
			details = append(details, "Размер: "+b.formatFileSize(book.FileSize))
		}

		if len(details) > 0 {
			content := strings.Join(details, "\n")
			if book.Annotation != "" {
				content = book.Annotation + "\n\n" + content
			}

			entry.Content = &Content{
				Type: "text",
				Text: content,
			}
		}
	}

	return entry
}

// getFileType returns MIME type for file format
func (b *Builder) getFileType(format string) string {
	switch strings.ToLower(format) {
	case "fb2":
		return TypeFB2
	case "epub":
		return TypeEPUB
	case "pdf":
		return TypePDF
	default:
		return "application/octet-stream"
	}
}

// buildPageURL builds URL with page parameter
func (b *Builder) buildPageURL(baseURL string, page int) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()

	return u.String()
}

// formatFileSize formats file size in human readable format
func (b *Builder) formatFileSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	k := int64(1024)
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0

	for bytes >= k && i < len(sizes)-1 {
		bytes /= k
		i++
	}

	return fmt.Sprintf("%d %s", bytes, sizes[i])
}