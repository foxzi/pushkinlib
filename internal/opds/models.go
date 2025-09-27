package opds

import (
	"encoding/xml"
	"time"
)

// Feed represents OPDS Atom feed
type Feed struct {
	XMLName   xml.Name `xml:"feed"`
	Xmlns     string   `xml:"xmlns,attr"`
	XmlnsDC   string   `xml:"xmlns:dc,attr"`
	XmlnsOPDS string   `xml:"xmlns:opds,attr"`

	ID      string    `xml:"id"`
	Title   string    `xml:"title"`
	Updated time.Time `xml:"updated"`
	Icon    string    `xml:"icon,omitempty"`

	Author *Person `xml:"author,omitempty"`
	Links  []Link  `xml:"link"`

	Entries []Entry `xml:"entry"`
}

// Entry represents OPDS entry (book or navigation)
type Entry struct {
	ID      string    `xml:"id"`
	Title   string    `xml:"title"`
	Updated time.Time `xml:"updated"`
	Summary string    `xml:"summary,omitempty"`
	Content *Content  `xml:"content,omitempty"`

	Authors    []Person   `xml:"author"`
	Categories []Category `xml:"category"`
	Links      []Link     `xml:"link"`

	// Dublin Core elements
	Language string `xml:"dc:language,omitempty"`
	Issued   string `xml:"dc:issued,omitempty"`
}

// Person represents author or contributor
type Person struct {
	Name string `xml:"name"`
	URI  string `xml:"uri,omitempty"`
}

// Link represents relation links
type Link struct {
	Rel      string `xml:"rel,attr"`
	Type     string `xml:"type,attr"`
	Href     string `xml:"href,attr"`
	Title    string `xml:"title,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Length   int64  `xml:"length,attr,omitempty"`
}

// Category represents genre/category
type Category struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
}

// Content represents entry content
type Content struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

// Constants for OPDS relations
const (
	// Navigation relations
	RelStart       = "start"
	RelUp          = "up"
	RelNext        = "next"
	RelPrev        = "prev"
	RelSubsection  = "subsection"
	RelSearch      = "search"

	// Acquisition relations
	RelAcquisition     = "http://opds-spec.org/acquisition"
	RelAcquisitionOpen = "http://opds-spec.org/acquisition/open-access"

	// Content types
	TypeNavigation = "application/atom+xml;profile=opds-catalog;kind=navigation"
	TypeAcquisition = "application/atom+xml;profile=opds-catalog;kind=acquisition"
	TypeSearch      = "application/opensearchdescription+xml"

	// File types
	TypeFB2  = "application/fb2+zip"
	TypeEPUB = "application/epub+zip"
	TypePDF  = "application/pdf"
)