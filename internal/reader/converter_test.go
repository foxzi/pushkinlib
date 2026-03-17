package reader

import (
	"strings"
	"testing"
)

func TestSectionToHTML_BasicParagraphs(t *testing.T) {
	sec := &FB2Section{
		Title: &FB2Title{
			Paragraphs: []FB2Paragraph{{Content: "Chapter Title"}},
		},
		Content: []FB2Block{
			{Paragraph: &FB2Paragraph{Content: "First paragraph."}},
			{Paragraph: &FB2Paragraph{Content: "Second with <emphasis>italic</emphasis>."}},
			{EmptyLine: true},
		},
	}

	html := SectionToHTML(sec, "book123")

	if !strings.Contains(html, "<h2 class=\"title\">Chapter Title</h2>") {
		t.Error("missing title h2")
	}
	if !strings.Contains(html, "<p>First paragraph.</p>") {
		t.Error("missing first paragraph")
	}
	if !strings.Contains(html, "<em>italic</em>") {
		t.Error("emphasis should be converted to <em>")
	}
	if !strings.Contains(html, "empty-line") {
		t.Error("missing empty-line div")
	}
}

func TestSectionToHTML_Poem(t *testing.T) {
	sec := &FB2Section{
		Content: []FB2Block{
			{Poem: &FB2Poem{
				Title: &FB2Title{Paragraphs: []FB2Paragraph{{Content: "Poem Title"}}},
				Stanzas: []FB2Stanza{
					{Verses: []FB2Verse{
						{Content: "Line one"},
						{Content: "Line two"},
					}},
				},
				TextAuthor: "Poet Name",
			}},
		},
	}

	html := SectionToHTML(sec, "book123")

	if !strings.Contains(html, "class=\"poem\"") {
		t.Error("missing poem class")
	}
	if !strings.Contains(html, "class=\"verse\"") {
		t.Error("missing verse class")
	}
	if !strings.Contains(html, "Poet Name") {
		t.Error("missing text-author")
	}
}

func TestSectionToHTML_Epigraph(t *testing.T) {
	sec := &FB2Section{
		Epigraphs: []FB2Epigraph{
			{
				Paragraphs: []FB2Paragraph{{Content: "A wise quote"}},
				TextAuthor: "Wise Person",
			},
		},
	}

	html := SectionToHTML(sec, "book123")

	if !strings.Contains(html, "class=\"epigraph\"") {
		t.Error("missing epigraph class")
	}
	if !strings.Contains(html, "Wise Person") {
		t.Error("missing text-author in epigraph")
	}
}

func TestSectionToHTML_Image(t *testing.T) {
	sec := &FB2Section{
		Image: &FB2Image{Href: "#cover.jpg"},
		Content: []FB2Block{
			{Image: &FB2Image{Href: "#fig1.png"}},
		},
	}

	html := SectionToHTML(sec, "mybook")

	if !strings.Contains(html, "/api/v1/books/mybook/image/cover.jpg") {
		t.Error("missing section-level image with correct URL")
	}
	if !strings.Contains(html, "/api/v1/books/mybook/image/fig1.png") {
		t.Error("missing inline image with correct URL")
	}
}

func TestSectionToHTML_Cite(t *testing.T) {
	sec := &FB2Section{
		Content: []FB2Block{
			{Cite: &FB2Cite{
				Paragraphs: []FB2Paragraph{{Content: "Quoted text."}},
				TextAuthor: "Author",
			}},
		},
	}

	html := SectionToHTML(sec, "book1")

	if !strings.Contains(html, "<blockquote class=\"cite\"") {
		t.Error("missing blockquote for cite")
	}
	if !strings.Contains(html, "Author") {
		t.Error("missing cite author")
	}
}

func TestSectionToHTML_Table(t *testing.T) {
	sec := &FB2Section{
		Content: []FB2Block{
			{Table: &FB2Table{
				Rows: []FB2TableRow{
					{Headers: []FB2TableCell{{Content: "Header 1"}}},
					{Cells: []FB2TableCell{{Content: "Cell 1", Align: "center"}}},
				},
			}},
		},
	}

	html := SectionToHTML(sec, "book1")

	if !strings.Contains(html, "<table class=\"fb2-table\"") {
		t.Error("missing table")
	}
	if !strings.Contains(html, "<th>") {
		t.Error("missing th")
	}
	if !strings.Contains(html, "text-align:center") {
		t.Error("missing cell alignment")
	}
}

func TestSectionToHTML_Subtitle(t *testing.T) {
	sec := &FB2Section{
		Content: []FB2Block{
			{Subtitle: "A Subtitle"},
		},
	}

	html := SectionToHTML(sec, "book1")

	if !strings.Contains(html, "<h3 class=\"subtitle\">A Subtitle</h3>") {
		t.Error("missing subtitle h3")
	}
}

func TestSectionToHTML_ParagraphID(t *testing.T) {
	sec := &FB2Section{
		Content: []FB2Block{
			{Paragraph: &FB2Paragraph{ID: "p42", Content: "Content"}},
		},
	}

	html := SectionToHTML(sec, "book1")

	if !strings.Contains(html, `id="p42"`) {
		t.Error("missing paragraph id attribute")
	}
}

func TestProcessInline_Emphasis(t *testing.T) {
	result := processInline("Hello <emphasis>world</emphasis>!", "b1")
	if result != "Hello <em>world</em>!" {
		t.Errorf("processInline emphasis = %q", result)
	}
}

func TestProcessInline_Strong(t *testing.T) {
	result := processInline("<strong>bold</strong>", "b1")
	if result != "<strong>bold</strong>" {
		t.Errorf("processInline strong = %q", result)
	}
}

func TestProcessLinks_External(t *testing.T) {
	input := `<a l:href="http://example.com">link</a>`
	result := processLinks(input, "b1")

	if !strings.Contains(result, `href="http://example.com"`) {
		t.Errorf("external link missing href: %s", result)
	}
	if !strings.Contains(result, `target="_blank"`) {
		t.Errorf("external link missing target: %s", result)
	}
}

func TestProcessLinks_FootnoteRef(t *testing.T) {
	input := `<a l:href="#note1" type="note">1</a>`
	result := processLinks(input, "b1")

	if !strings.Contains(result, `class="footnote-ref"`) {
		t.Errorf("footnote ref missing class: %s", result)
	}
	if !strings.Contains(result, `data-note="note1"`) {
		t.Errorf("footnote ref missing data-note: %s", result)
	}
}

func TestProcessLinks_InternalLink(t *testing.T) {
	input := `<a l:href="#section5">see here</a>`
	result := processLinks(input, "b1")

	if !strings.Contains(result, `href="#section5"`) {
		t.Errorf("internal link missing href: %s", result)
	}
	if strings.Contains(result, "footnote-ref") {
		t.Errorf("non-note internal link should not have footnote-ref class")
	}
}

func TestProcessInlineImages(t *testing.T) {
	input := `text <image l:href="#fig1.png"/> more text`
	result := processInlineImages(input, "book42")

	if !strings.Contains(result, "/api/v1/books/book42/image/fig1.png") {
		t.Errorf("inline image URL wrong: %s", result)
	}
	if !strings.Contains(result, "class=\"inline-image\"") {
		t.Errorf("inline image missing class: %s", result)
	}
}

func TestExtractAttr(t *testing.T) {
	tests := []struct {
		tag, attr, want string
	}{
		{`<a l:href="#note1" type="note">`, "l:href", "#note1"},
		{`<a l:href="#note1" type="note">`, "type", "note"},
		{`<a l:href='#note1'>`, "l:href", "#note1"},
		{`<image xlink:href="#img"/>`, "xlink:href", "#img"},
		{`<a>`, "href", ""},
	}

	for _, tt := range tests {
		got := extractAttr(tt.tag, tt.attr)
		if got != tt.want {
			t.Errorf("extractAttr(%q, %q) = %q, want %q", tt.tag, tt.attr, got, tt.want)
		}
	}
}

func TestSectionToHTML_FullRoundTrip(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	flat := FlattenSections(book)
	if len(flat) == 0 {
		t.Fatal("no sections")
	}

	for _, sec := range flat {
		html := SectionToHTML(sec.Section, "test-book")
		if html == "" {
			t.Errorf("section %d %q produced empty HTML", sec.Index, sec.Title)
		}
	}
}
