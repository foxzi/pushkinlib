package reader

import (
	"strings"
	"testing"
)

const sampleFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
 <description>
  <title-info>
   <genre>sf</genre>
   <author><first-name>Test</first-name><last-name>Author</last-name></author>
   <book-title>Test Book</book-title>
   <lang>ru</lang>
  </title-info>
 </description>
 <body>
  <title><p>Test Book Title</p></title>
  <section>
   <title><p>Chapter 1</p></title>
   <p>First paragraph of chapter one.</p>
   <p>Second paragraph with <emphasis>emphasis</emphasis> and <strong>bold</strong>.</p>
   <empty-line/>
   <p>After empty line.</p>
  </section>
  <section>
   <title><p>Chapter 2</p></title>
   <epigraph>
    <p>To be or not to be</p>
    <text-author>Shakespeare</text-author>
   </epigraph>
   <p>Chapter two content.</p>
   <poem>
    <title><p>A Poem</p></title>
    <stanza>
     <v>First line</v>
     <v>Second line</v>
    </stanza>
    <text-author>Poet</text-author>
   </poem>
   <cite>
    <p>A quoted passage.</p>
    <text-author>Quotee</text-author>
   </cite>
   <subtitle>A subtitle</subtitle>
   <section>
    <title><p>Subsection 2.1</p></title>
    <p>Subsection content.</p>
   </section>
  </section>
 </body>
 <body name="notes">
  <section id="note1">
   <title><p>1</p></title>
   <p>This is a footnote.</p>
  </section>
 </body>
 <binary id="cover.jpg" content-type="image/jpeg">dGVzdA==</binary>
</FictionBook>`

func TestParseFB2_Basic(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	if len(book.Bodies) != 2 {
		t.Errorf("expected 2 bodies, got %d", len(book.Bodies))
	}

	if len(book.Binaries) != 1 {
		t.Errorf("expected 1 binary, got %d", len(book.Binaries))
	}

	// Main body
	main := book.Bodies[0]
	if main.Name != "" {
		t.Errorf("main body name should be empty, got %q", main.Name)
	}
	if main.Title == nil || len(main.Title.Paragraphs) != 1 {
		t.Fatal("main body should have title with 1 paragraph")
	}
	if len(main.Sections) != 2 {
		t.Errorf("main body should have 2 sections, got %d", len(main.Sections))
	}

	// Notes body
	notes := book.Bodies[1]
	if notes.Name != "notes" {
		t.Errorf("expected notes body, got %q", notes.Name)
	}
	if len(notes.Sections) != 1 {
		t.Errorf("expected 1 note section, got %d", len(notes.Sections))
	}
}

func TestParseFB2_SectionContent(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	ch1 := book.Bodies[0].Sections[0]
	if ch1.Title == nil {
		t.Fatal("chapter 1 should have title")
	}
	if extractTitleText(ch1.Title) != "Chapter 1" {
		t.Errorf("chapter 1 title = %q, want %q", extractTitleText(ch1.Title), "Chapter 1")
	}

	// Chapter 1: 3 paragraphs + 1 empty-line = 4 content blocks
	if len(ch1.Content) != 4 {
		t.Errorf("chapter 1 content blocks = %d, want 4", len(ch1.Content))
	}

	// First block is paragraph
	if ch1.Content[0].Paragraph == nil {
		t.Error("first block should be paragraph")
	}

	// Third block is empty-line
	if !ch1.Content[2].EmptyLine {
		t.Error("third block should be empty-line")
	}

	// Chapter 2 with nested content types
	ch2 := book.Bodies[0].Sections[1]
	if len(ch2.Epigraphs) != 1 {
		t.Errorf("chapter 2 epigraphs = %d, want 1", len(ch2.Epigraphs))
	}
	if ch2.Epigraphs[0].TextAuthor != "Shakespeare" {
		t.Errorf("epigraph author = %q, want %q", ch2.Epigraphs[0].TextAuthor, "Shakespeare")
	}

	// Chapter 2 content: p, poem, cite, subtitle = 4 blocks
	if len(ch2.Content) != 4 {
		t.Errorf("chapter 2 content blocks = %d, want 4", len(ch2.Content))
	}

	// Poem
	if ch2.Content[1].Poem == nil {
		t.Error("second block should be poem")
	}
	poem := ch2.Content[1].Poem
	if len(poem.Stanzas) != 1 || len(poem.Stanzas[0].Verses) != 2 {
		t.Errorf("poem should have 1 stanza with 2 verses")
	}
	if poem.TextAuthor != "Poet" {
		t.Errorf("poem author = %q, want %q", poem.TextAuthor, "Poet")
	}

	// Cite
	if ch2.Content[2].Cite == nil {
		t.Error("third block should be cite")
	}

	// Subtitle
	if ch2.Content[3].Subtitle == "" {
		t.Error("fourth block should be subtitle")
	}

	// Subsections
	if len(ch2.Sections) != 1 {
		t.Errorf("chapter 2 subsections = %d, want 1", len(ch2.Sections))
	}
}

func TestParseFB2_Binary(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	bin := book.Binaries[0]
	if bin.ID != "cover.jpg" {
		t.Errorf("binary ID = %q, want %q", bin.ID, "cover.jpg")
	}
	if bin.ContentType != "image/jpeg" {
		t.Errorf("binary content-type = %q, want %q", bin.ContentType, "image/jpeg")
	}
	if bin.Data != "dGVzdA==" {
		t.Errorf("binary data = %q, want %q", bin.Data, "dGVzdA==")
	}
}

func TestParseFB2_NoBody(t *testing.T) {
	xml := `<?xml version="1.0"?><FictionBook><description></description></FictionBook>`
	_, err := ParseFB2(strings.NewReader(xml))
	if err == nil {
		t.Error("expected error for FB2 with no body")
	}
}

func TestParseFB2_Windows1251(t *testing.T) {
	// Minimal windows-1251 encoded FB2 (the header declares encoding)
	xml := `<?xml version="1.0" encoding="windows-1251"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
 <description><title-info><book-title>Test</book-title><lang>ru</lang></title-info></description>
 <body><section><p>Hello</p></section></body>
</FictionBook>`
	book, err := ParseFB2(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("ParseFB2 windows-1251 failed: %v", err)
	}
	if len(book.Bodies) != 1 {
		t.Errorf("expected 1 body, got %d", len(book.Bodies))
	}
}

func TestFlattenSections(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	flat := FlattenSections(book)

	// Chapter 1 (leaf), Chapter 2 (has content + subsections), Subsection 2.1, Note 1
	// Chapter 2 has its own content AND sub-sections, so it appears as a leaf too
	expectedCount := 4
	if len(flat) != expectedCount {
		t.Errorf("flat sections = %d, want %d", len(flat), expectedCount)
		for _, s := range flat {
			t.Logf("  [%d] L%d %q body=%q", s.Index, s.Level, s.Title, s.BodyName)
		}
	}

	// Check ordering
	if flat[0].Title != "Chapter 1" {
		t.Errorf("flat[0] title = %q, want %q", flat[0].Title, "Chapter 1")
	}
	if flat[0].Level != 0 {
		t.Errorf("flat[0] level = %d, want 0", flat[0].Level)
	}

	// Notes body section
	last := flat[len(flat)-1]
	if last.BodyName != "notes" {
		t.Errorf("last section body_name = %q, want %q", last.BodyName, "notes")
	}
}

func TestBuildTOC(t *testing.T) {
	book, err := ParseFB2(strings.NewReader(sampleFB2))
	if err != nil {
		t.Fatalf("ParseFB2 failed: %v", err)
	}

	flat := FlattenSections(book)
	toc := BuildTOC(flat)

	if len(toc) != len(flat) {
		t.Errorf("TOC entries = %d, want %d", len(toc), len(flat))
	}

	for i, entry := range toc {
		if entry.Section != flat[i].Index {
			t.Errorf("TOC[%d] section = %d, want %d", i, entry.Section, flat[i].Index)
		}
	}
}

func TestStripXMLTags(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"<em>hello</em>", "hello"},
		{"a <strong>bold</strong> word", "a bold word"},
		{"<a href=\"#x\">link</a>", "link"},
		{"", ""},
	}

	for _, tt := range tests {
		got := stripXMLTags(tt.input)
		if got != tt.want {
			t.Errorf("stripXMLTags(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
