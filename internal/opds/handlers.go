package opds

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/storage"
)

// Handler handles OPDS requests
type Handler struct {
	repo    *storage.Repository
	builder *Builder
}

// NewHandler creates a new OPDS handler
func NewHandler(repo *storage.Repository, baseURL, catalogTitle string) *Handler {
	return &Handler{
		repo:    repo,
		builder: NewBuilder(baseURL, catalogTitle),
	}
}

// Root serves the root OPDS catalog
func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	feed := h.builder.BuildRootFeed()
	h.writeFeed(w, feed)
}

// NewBooks serves newest books
func (h *Handler) NewBooks(w http.ResponseWriter, r *http.Request) {
	page := h.getPageFromQuery(r)
	pageSize := 30

	filter := storage.BookFilter{
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		SortBy:    "date_added",
		SortOrder: "desc",
	}

	result, err := h.repo.SearchBooks(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	feedID := h.builder.baseURL + "/opds/books/new"
	if page > 1 {
		feedID += "?page=" + strconv.Itoa(page)
	}

	feed := h.builder.BuildBooksFeed(result.Books, "Новые поступления", feedID, page, result.Total)
	h.writeFeed(w, feed)
}

// SearchBooks handles OPDS search
func (h *Handler) SearchBooks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	page := h.getPageFromQuery(r)
	pageSize := 30

	filter := storage.BookFilter{
		Query:     query,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		SortBy:    "relevance",
		SortOrder: "asc",
	}

	result, err := h.repo.SearchBooks(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := "Результаты поиска"
	if query != "" {
		title = fmt.Sprintf("Поиск: %s", query)
	}

	feedID := h.builder.baseURL + "/opds/search"
	if query != "" {
		feedID += "?q=" + query
	}
	if page > 1 {
		separator := "?"
		if query != "" {
			separator = "&"
		}
		feedID += separator + "page=" + strconv.Itoa(page)
	}

	feed := h.builder.BuildBooksFeed(result.Books, title, feedID, page, result.Total)
	h.writeFeed(w, feed)
}

// Authors serves authors catalog (navigation)
func (h *Handler) Authors(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement authors navigation
	h.notImplemented(w, "Authors catalog")
}

// Series serves series catalog (navigation)
func (h *Handler) Series(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement series navigation
	h.notImplemented(w, "Series catalog")
}

// Genres serves genres catalog (navigation)
func (h *Handler) Genres(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement genres navigation
	h.notImplemented(w, "Genres catalog")
}

// BooksByAuthor serves books by specific author
func (h *Handler) BooksByAuthor(w http.ResponseWriter, r *http.Request) {
	authorID := chi.URLParam(r, "id")
	page := h.getPageFromQuery(r)
	pageSize := 30

	// TODO: Implement filtering by author ID
	filter := storage.BookFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}

	result, err := h.repo.SearchBooks(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := "Книги автора"
	feedID := h.builder.baseURL + "/opds/authors/" + authorID

	feed := h.builder.BuildBooksFeed(result.Books, title, feedID, page, result.Total)
	h.writeFeed(w, feed)
}

// OpenSearch serves OpenSearch description
func (h *Handler) OpenSearch(w http.ResponseWriter, r *http.Request) {
	description := `<?xml version="1.0" encoding="UTF-8"?>
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/">
    <ShortName>` + h.builder.catalogTitle + `</ShortName>
    <Description>Поиск книг в каталоге ` + h.builder.catalogTitle + `</Description>
    <Tags>books library catalog</Tags>
    <Contact>admin@example.com</Contact>
    <Url type="application/atom+xml;profile=opds-catalog"
         template="` + h.builder.baseURL + `/opds/search?q={searchTerms}"/>
    <LongName>` + h.builder.catalogTitle + ` - поиск книг</LongName>
    <Image height="64" width="64" type="image/png">` + h.builder.baseURL + `/favicon.ico</Image>
    <Query role="example" searchTerms="фантастика"/>
    <Developer>Pushkinlib</Developer>
    <Attribution>Pushkinlib OPDS catalog</Attribution>
    <SyndicationRight>open</SyndicationRight>
    <AdultContent>false</AdultContent>
    <Language>ru-ru</Language>
    <InputEncoding>UTF-8</InputEncoding>
    <OutputEncoding>UTF-8</OutputEncoding>
</OpenSearchDescription>`

	w.Header().Set("Content-Type", "application/opensearchdescription+xml; charset=utf-8")
	w.Write([]byte(description))
}

// getPageFromQuery extracts page number from query parameters
func (h *Handler) getPageFromQuery(r *http.Request) int {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		return 1
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}

	return page
}

// writeFeed writes OPDS feed as XML
func (h *Handler) writeFeed(w http.ResponseWriter, feed *Feed) {
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Write XML header
	w.Write([]byte(xml.Header))

	// Encode feed
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(feed); err != nil {
		http.Error(w, "Failed to encode feed", http.StatusInternalServerError)
		return
	}
}

// notImplemented serves a placeholder feed for not implemented features
func (h *Handler) notImplemented(w http.ResponseWriter, feature string) {
	feed := &Feed{
		Xmlns:     "http://www.w3.org/2005/Atom",
		XmlnsDC:   "http://purl.org/dc/terms/",
		XmlnsOPDS: "http://opds-spec.org/2010/catalog",

		ID:      h.builder.baseURL + "/opds/not-implemented",
		Title:   feature + " (В разработке)",
		Updated: time.Now(),

		Author: &Person{
			Name: h.builder.catalogTitle,
		},

		Links: []Link{
			{
				Rel:  RelStart,
				Type: TypeNavigation,
				Href: h.builder.baseURL + "/opds",
			},
			{
				Rel:  RelUp,
				Type: TypeNavigation,
				Href: h.builder.baseURL + "/opds",
			},
		},

		Entries: []Entry{
			{
				ID:      h.builder.baseURL + "/opds/not-implemented",
				Title:   "Функция в разработке",
				Updated: time.Now(),
				Summary: fmt.Sprintf("Раздел '%s' будет реализован в следующих версиях.", feature),
			},
		},
	}

	h.writeFeed(w, feed)
}