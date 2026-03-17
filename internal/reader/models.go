package reader

// FB2Book represents a fully parsed FB2 document for reading
type FB2Book struct {
	Bodies   []FB2Body   `xml:"body"`
	Binaries []FB2Binary `xml:"binary"`
}

// FB2Body represents <body> element — main text or named (e.g. "notes", "footnotes")
type FB2Body struct {
	Name      string        `xml:"name,attr,omitempty"`
	Title     *FB2Title     `xml:"title,omitempty"`
	Epigraphs []FB2Epigraph `xml:"epigraph,omitempty"`
	Sections  []FB2Section  `xml:"section"`
}

// FB2Section represents <section> — a chapter or subchapter
type FB2Section struct {
	ID         string             `xml:"id,attr,omitempty"`
	Title      *FB2Title          `xml:"title,omitempty"`
	Epigraphs  []FB2Epigraph      `xml:"epigraph,omitempty"`
	Image      *FB2Image          `xml:"image,omitempty"`
	Annotation *FB2AnnotationBody `xml:"annotation,omitempty"`
	Content    []FB2Block         `xml:"-"` // Mixed content parsed manually
	Sections   []FB2Section       `xml:"section"`
}

// FB2Title represents <title> element
type FB2Title struct {
	Paragraphs []FB2Paragraph `xml:"p"`
	EmptyLines []struct{}     `xml:"empty-line"`
}

// FB2Epigraph represents <epigraph> element
type FB2Epigraph struct {
	ID         string         `xml:"id,attr,omitempty"`
	Paragraphs []FB2Paragraph `xml:"p"`
	Poems      []FB2Poem      `xml:"poem"`
	Cites      []FB2Cite      `xml:"cite"`
	TextAuthor string         `xml:"text-author,omitempty"`
}

// FB2Paragraph represents <p> element with mixed inline content
type FB2Paragraph struct {
	ID      string `xml:"id,attr,omitempty"`
	Content string `xml:",innerxml"`
}

// FB2Poem represents <poem> element
type FB2Poem struct {
	ID         string        `xml:"id,attr,omitempty"`
	Title      *FB2Title     `xml:"title,omitempty"`
	Epigraphs  []FB2Epigraph `xml:"epigraph,omitempty"`
	Stanzas    []FB2Stanza   `xml:"stanza"`
	TextAuthor string        `xml:"text-author,omitempty"`
	Date       string        `xml:"date,omitempty"`
}

// FB2Stanza represents <stanza> element
type FB2Stanza struct {
	Title    *FB2Title  `xml:"title,omitempty"`
	Subtitle string     `xml:"subtitle,omitempty"`
	Verses   []FB2Verse `xml:"v"`
}

// FB2Verse represents <v> element (verse line)
type FB2Verse struct {
	ID      string `xml:"id,attr,omitempty"`
	Content string `xml:",innerxml"`
}

// FB2Cite represents <cite> element
type FB2Cite struct {
	ID         string         `xml:"id,attr,omitempty"`
	Paragraphs []FB2Paragraph `xml:"p"`
	Poems      []FB2Poem      `xml:"poem"`
	Subtitle   string         `xml:"subtitle,omitempty"`
	TextAuthor string         `xml:"text-author,omitempty"`
}

// FB2Image represents <image> element
type FB2Image struct {
	Href string `xml:"href,attr"`
	Alt  string `xml:"alt,attr,omitempty"`
}

// FB2Table represents <table> element
type FB2Table struct {
	ID   string        `xml:"id,attr,omitempty"`
	Rows []FB2TableRow `xml:"tr"`
}

// FB2TableRow represents <tr> element
type FB2TableRow struct {
	Cells   []FB2TableCell `xml:"td"`
	Headers []FB2TableCell `xml:"th"`
}

// FB2TableCell represents <td> or <th> element
type FB2TableCell struct {
	Content string `xml:",innerxml"`
	Align   string `xml:"align,attr,omitempty"`
	Colspan string `xml:"colspan,attr,omitempty"`
	Rowspan string `xml:"rowspan,attr,omitempty"`
}

// FB2AnnotationBody represents <annotation> inside body sections
type FB2AnnotationBody struct {
	Paragraphs []FB2Paragraph `xml:"p"`
}

// FB2Binary represents <binary> element (embedded images)
type FB2Binary struct {
	ID          string `xml:"id,attr"`
	ContentType string `xml:"content-type,attr"`
	Data        string `xml:",chardata"`
}

// FB2Block is a union type for block-level content elements within a section.
// Only one field will be set.
type FB2Block struct {
	Paragraph *FB2Paragraph
	Poem      *FB2Poem
	Subtitle  string
	Cite      *FB2Cite
	EmptyLine bool
	Image     *FB2Image
	Table     *FB2Table
}

// TOCEntry represents a table-of-contents entry
type TOCEntry struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Level    int        `json:"level"`
	Section  int        `json:"section"` // flat section index for API
	Children []TOCEntry `json:"children,omitempty"`
}
