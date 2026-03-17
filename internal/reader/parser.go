package reader

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

// ParseFB2 parses an FB2 file and returns bodies and binaries.
// It handles both UTF-8 and windows-1251 encoded files.
func ParseFB2(r io.Reader) (*FB2Book, error) {
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel

	book := &FB2Book{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml token error: %w", err)
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "body":
			body, err := parseBody(decoder, &start)
			if err != nil {
				return nil, fmt.Errorf("parse body: %w", err)
			}
			book.Bodies = append(book.Bodies, *body)

		case "binary":
			var bin FB2Binary
			if err := decoder.DecodeElement(&bin, &start); err != nil {
				return nil, fmt.Errorf("parse binary: %w", err)
			}
			// Clean whitespace from base64 data
			bin.Data = strings.Join(strings.Fields(bin.Data), "")
			book.Binaries = append(book.Binaries, bin)

		case "description":
			// Skip description — it's handled by metadata extractor
			if err := decoder.Skip(); err != nil {
				return nil, fmt.Errorf("skip description: %w", err)
			}
		}
	}

	if len(book.Bodies) == 0 {
		return nil, fmt.Errorf("no <body> found in FB2 document")
	}

	return book, nil
}

// parseBody parses a <body> element.
func parseBody(decoder *xml.Decoder, start *xml.StartElement) (*FB2Body, error) {
	body := &FB2Body{}

	for _, attr := range start.Attr {
		if attr.Name.Local == "name" {
			body.Name = attr.Value
		}
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("body token: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "title":
				title, err := parseTitle(decoder)
				if err != nil {
					return nil, err
				}
				body.Title = title

			case "epigraph":
				var ep FB2Epigraph
				if err := decoder.DecodeElement(&ep, &t); err != nil {
					return nil, fmt.Errorf("decode epigraph: %w", err)
				}
				body.Epigraphs = append(body.Epigraphs, ep)

			case "section":
				sec, err := parseSection(decoder, &t)
				if err != nil {
					return nil, err
				}
				body.Sections = append(body.Sections, *sec)

			default:
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			if t.Name.Local == "body" {
				return body, nil
			}
		}
	}
}

// parseSection parses a <section> element with its mixed content.
// Sections can contain: title, epigraph, image, annotation, p, poem, subtitle,
// cite, empty-line, table — and nested sections.
func parseSection(decoder *xml.Decoder, start *xml.StartElement) (*FB2Section, error) {
	sec := &FB2Section{}

	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			sec.ID = attr.Value
		}
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("section token: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "title":
				title, err := parseTitle(decoder)
				if err != nil {
					return nil, err
				}
				sec.Title = title

			case "epigraph":
				var ep FB2Epigraph
				if err := decoder.DecodeElement(&ep, &t); err != nil {
					return nil, fmt.Errorf("decode epigraph: %w", err)
				}
				sec.Epigraphs = append(sec.Epigraphs, ep)

			case "image":
				img := parseImage(&t)
				// Image can be at section level (block) or first element
				if sec.Image == nil && len(sec.Content) == 0 {
					sec.Image = img
				} else {
					sec.Content = append(sec.Content, FB2Block{Image: img})
				}
				if err := decoder.Skip(); err != nil {
					return nil, err
				}

			case "annotation":
				var ann FB2AnnotationBody
				if err := decoder.DecodeElement(&ann, &t); err != nil {
					return nil, fmt.Errorf("decode annotation: %w", err)
				}
				sec.Annotation = &ann

			case "p":
				var p FB2Paragraph
				if err := decoder.DecodeElement(&p, &t); err != nil {
					return nil, fmt.Errorf("decode p: %w", err)
				}
				sec.Content = append(sec.Content, FB2Block{Paragraph: &p})

			case "poem":
				var poem FB2Poem
				if err := decoder.DecodeElement(&poem, &t); err != nil {
					return nil, fmt.Errorf("decode poem: %w", err)
				}
				sec.Content = append(sec.Content, FB2Block{Poem: &poem})

			case "subtitle":
				var sub struct {
					Content string `xml:",innerxml"`
				}
				if err := decoder.DecodeElement(&sub, &t); err != nil {
					return nil, fmt.Errorf("decode subtitle: %w", err)
				}
				sec.Content = append(sec.Content, FB2Block{Subtitle: sub.Content})

			case "cite":
				var cite FB2Cite
				if err := decoder.DecodeElement(&cite, &t); err != nil {
					return nil, fmt.Errorf("decode cite: %w", err)
				}
				sec.Content = append(sec.Content, FB2Block{Cite: &cite})

			case "empty-line":
				sec.Content = append(sec.Content, FB2Block{EmptyLine: true})
				if err := decoder.Skip(); err != nil {
					return nil, err
				}

			case "table":
				var tbl FB2Table
				if err := decoder.DecodeElement(&tbl, &t); err != nil {
					return nil, fmt.Errorf("decode table: %w", err)
				}
				sec.Content = append(sec.Content, FB2Block{Table: &tbl})

			case "section":
				sub, err := parseSection(decoder, &t)
				if err != nil {
					return nil, err
				}
				sec.Sections = append(sec.Sections, *sub)

			default:
				// Unknown element — skip
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			if t.Name.Local == "section" {
				return sec, nil
			}
		}
	}
}

// parseTitle parses <title> element content (sequence of <p> and <empty-line>).
func parseTitle(decoder *xml.Decoder) (*FB2Title, error) {
	title := &FB2Title{}

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("title token: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				var p FB2Paragraph
				if err := decoder.DecodeElement(&p, &t); err != nil {
					return nil, fmt.Errorf("decode title p: %w", err)
				}
				title.Paragraphs = append(title.Paragraphs, p)

			case "empty-line":
				title.EmptyLines = append(title.EmptyLines, struct{}{})
				if err := decoder.Skip(); err != nil {
					return nil, err
				}

			default:
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			if t.Name.Local == "title" {
				return title, nil
			}
		}
	}
}

// parseImage extracts href and alt from <image> start element attributes.
func parseImage(start *xml.StartElement) *FB2Image {
	img := &FB2Image{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "href":
			img.Href = attr.Value
		case "alt":
			img.Alt = attr.Value
		}
	}
	return img
}

// FlatSection is a flattened section with its position in the tree,
// used for paginated content delivery via the API.
type FlatSection struct {
	Index    int         `json:"index"`
	Title    string      `json:"title"`
	Level    int         `json:"level"`
	BodyName string      `json:"body_name,omitempty"`
	Section  *FB2Section `json:"-"`
}

// FlattenSections returns a flat list of all leaf/content sections in the book.
// This assigns a sequential index to each section for API pagination.
func FlattenSections(book *FB2Book) []FlatSection {
	var result []FlatSection
	idx := 0

	for _, body := range book.Bodies {
		for i := range body.Sections {
			flattenSection(&body.Sections[i], body.Name, 0, &idx, &result)
		}
	}

	return result
}

// flattenSection recursively flattens sections.
// Sections with content (paragraphs, poems, etc.) become entries.
// Sections with only sub-sections are structural and are not leaf entries themselves.
func flattenSection(sec *FB2Section, bodyName string, level int, idx *int, result *[]FlatSection) {
	title := extractTitleText(sec.Title)

	if len(sec.Sections) > 0 {
		// Has sub-sections — recurse into them.
		// If this section also has its own content, include it as a leaf too.
		if len(sec.Content) > 0 || sec.Image != nil || sec.Annotation != nil {
			*result = append(*result, FlatSection{
				Index:    *idx,
				Title:    title,
				Level:    level,
				BodyName: bodyName,
				Section:  sec,
			})
			*idx++
		}
		for i := range sec.Sections {
			flattenSection(&sec.Sections[i], bodyName, level+1, idx, result)
		}
	} else {
		// Leaf section
		*result = append(*result, FlatSection{
			Index:    *idx,
			Title:    title,
			Level:    level,
			BodyName: bodyName,
			Section:  sec,
		})
		*idx++
	}
}

// extractTitleText extracts plain text from a title element.
func extractTitleText(title *FB2Title) string {
	if title == nil {
		return ""
	}
	var parts []string
	for _, p := range title.Paragraphs {
		text := stripXMLTags(p.Content)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " / ")
}

// stripXMLTags removes XML/HTML tags from a string, leaving only text content.
func stripXMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// BuildTOC generates a table of contents from flattened sections.
func BuildTOC(sections []FlatSection) []TOCEntry {
	var entries []TOCEntry
	for _, s := range sections {
		entries = append(entries, TOCEntry{
			ID:      s.Section.ID,
			Title:   s.Title,
			Level:   s.Level,
			Section: s.Index,
		})
	}
	return entries
}
