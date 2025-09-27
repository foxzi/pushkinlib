package metadata

import "time"

// BookMetadata represents extracted book metadata
type BookMetadata struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Authors     []string  `json:"authors"`
	Series      string    `json:"series,omitempty"`
	SeriesNum   int       `json:"series_num,omitempty"`
	Genres      []string  `json:"genres"`
	Year        int       `json:"year,omitempty"`
	Language    string    `json:"language"`
	Annotation  string    `json:"annotation,omitempty"`
	Keywords    []string  `json:"keywords,omitempty"`
	Date        time.Time `json:"date"`

	// File info
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	Format      string `json:"format"` // fb2, epub, etc

	// Archive info (for generated archives)
	ArchivePath string `json:"archive_path,omitempty"`
	FileNum     string `json:"file_num,omitempty"`
}

// FB2Description represents FB2 book description
type FB2Description struct {
	TitleInfo   FB2TitleInfo   `xml:"title-info"`
	SrcTitleInfo *FB2TitleInfo `xml:"src-title-info,omitempty"`
	DocumentInfo FB2DocumentInfo `xml:"document-info"`
	PublishInfo *FB2PublishInfo `xml:"publish-info,omitempty"`
}

// FB2TitleInfo represents FB2 title information
type FB2TitleInfo struct {
	Genres      []FB2Genre      `xml:"genre"`
	Authors     []FB2Author     `xml:"author"`
	BookTitle   string          `xml:"book-title"`
	Annotation  *FB2Annotation  `xml:"annotation,omitempty"`
	Keywords    string          `xml:"keywords,omitempty"`
	Date        *FB2Date        `xml:"date,omitempty"`
	Lang        string          `xml:"lang"`
	SrcLang     string          `xml:"src-lang,omitempty"`
	Translators []FB2Author     `xml:"translator,omitempty"`
	Sequence    *FB2Sequence    `xml:"sequence,omitempty"`
}

// FB2Author represents FB2 author
type FB2Author struct {
	FirstName  string `xml:"first-name,omitempty"`
	MiddleName string `xml:"middle-name,omitempty"`
	LastName   string `xml:"last-name,omitempty"`
	Nickname   string `xml:"nickname,omitempty"`
	HomePage   string `xml:"home-page,omitempty"`
	Email      string `xml:"email,omitempty"`
}

// FB2Genre represents FB2 genre
type FB2Genre struct {
	Value string `xml:",chardata"`
}

// FB2Annotation represents FB2 annotation
type FB2Annotation struct {
	Content string `xml:",innerxml"`
}

// FB2Date represents FB2 date
type FB2Date struct {
	Value string `xml:"value,attr,omitempty"`
	Text  string `xml:",chardata"`
}

// FB2Sequence represents FB2 sequence
type FB2Sequence struct {
	Name   string `xml:"name,attr"`
	Number string `xml:"number,attr,omitempty"`
}

// FB2DocumentInfo represents FB2 document info
type FB2DocumentInfo struct {
	Authors  []FB2Author `xml:"author"`
	Date     *FB2Date    `xml:"date,omitempty"`
	ID       string      `xml:"id,omitempty"`
	Version  string      `xml:"version,omitempty"`
}

// FB2PublishInfo represents FB2 publish info
type FB2PublishInfo struct {
	BookName  string `xml:"book-name,omitempty"`
	Publisher string `xml:"publisher,omitempty"`
	City      string `xml:"city,omitempty"`
	Year      string `xml:"year,omitempty"`
	ISBN      string `xml:"isbn,omitempty"`
}