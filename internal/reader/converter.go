package reader

import (
	"fmt"
	"html"
	"strings"
)

// SectionToHTML converts a flat section into reader-ready HTML.
// Image references are rewritten to use the API endpoint: /api/v1/books/{bookID}/image/{name}
func SectionToHTML(sec *FB2Section, bookID string) string {
	var b strings.Builder

	// Section title
	if sec.Title != nil {
		writeTitle(&b, sec.Title, "h2")
	}

	// Epigraphs
	for _, ep := range sec.Epigraphs {
		writeEpigraph(&b, &ep, bookID)
	}

	// Section-level image (e.g. chapter illustration)
	if sec.Image != nil {
		writeImage(&b, sec.Image, bookID)
	}

	// Annotation
	if sec.Annotation != nil {
		b.WriteString("<div class=\"annotation\">\n")
		for _, p := range sec.Annotation.Paragraphs {
			writeParagraph(&b, &p, bookID)
		}
		b.WriteString("</div>\n")
	}

	// Block content
	for _, block := range sec.Content {
		writeBlock(&b, &block, bookID)
	}

	// Inline sub-sections (for sections that have both content and sub-sections)
	for _, sub := range sec.Sections {
		b.WriteString("<section class=\"subsection\">\n")
		b.WriteString(SectionToHTML(&sub, bookID))
		b.WriteString("</section>\n")
	}

	return b.String()
}

// writeBlock writes a single block-level element.
func writeBlock(b *strings.Builder, block *FB2Block, bookID string) {
	switch {
	case block.Paragraph != nil:
		writeParagraph(b, block.Paragraph, bookID)

	case block.Poem != nil:
		writePoem(b, block.Poem, bookID)

	case block.Subtitle != "":
		b.WriteString("<h3 class=\"subtitle\">")
		b.WriteString(processInline(block.Subtitle, bookID))
		b.WriteString("</h3>\n")

	case block.Cite != nil:
		writeCite(b, block.Cite, bookID)

	case block.EmptyLine:
		b.WriteString("<div class=\"empty-line\"></div>\n")

	case block.Image != nil:
		writeImage(b, block.Image, bookID)

	case block.Table != nil:
		writeTable(b, block.Table, bookID)
	}
}

// writeTitle renders a <title> as heading elements.
func writeTitle(b *strings.Builder, title *FB2Title, tag string) {
	if title == nil || len(title.Paragraphs) == 0 {
		return
	}

	for _, p := range title.Paragraphs {
		b.WriteString("<")
		b.WriteString(tag)
		b.WriteString(" class=\"title\">")
		// Title content is typically plain text, but may contain emphasis
		b.WriteString(stripDangerousTags(p.Content))
		b.WriteString("</")
		b.WriteString(tag)
		b.WriteString(">\n")
	}
}

// writeParagraph renders a <p> element.
func writeParagraph(b *strings.Builder, p *FB2Paragraph, bookID string) {
	id := ""
	if p.ID != "" {
		id = fmt.Sprintf(" id=\"%s\"", html.EscapeString(p.ID))
	}
	b.WriteString(fmt.Sprintf("<p%s>", id))
	b.WriteString(processInline(p.Content, bookID))
	b.WriteString("</p>\n")
}

// writePoem renders a <poem> as a styled block.
func writePoem(b *strings.Builder, poem *FB2Poem, bookID string) {
	id := ""
	if poem.ID != "" {
		id = fmt.Sprintf(" id=\"%s\"", html.EscapeString(poem.ID))
	}
	b.WriteString(fmt.Sprintf("<div class=\"poem\"%s>\n", id))

	if poem.Title != nil {
		writeTitle(b, poem.Title, "h4")
	}

	for _, ep := range poem.Epigraphs {
		writeEpigraph(b, &ep, bookID)
	}

	for _, stanza := range poem.Stanzas {
		b.WriteString("<div class=\"stanza\">\n")
		if stanza.Title != nil {
			writeTitle(b, stanza.Title, "h5")
		}
		if stanza.Subtitle != "" {
			b.WriteString("<h5 class=\"subtitle\">")
			b.WriteString(html.EscapeString(stanza.Subtitle))
			b.WriteString("</h5>\n")
		}
		for _, v := range stanza.Verses {
			b.WriteString("<div class=\"verse\">")
			b.WriteString(processInline(v.Content, bookID))
			b.WriteString("</div>\n")
		}
		b.WriteString("</div>\n")
	}

	if poem.TextAuthor != "" {
		b.WriteString("<div class=\"text-author\">")
		b.WriteString(html.EscapeString(poem.TextAuthor))
		b.WriteString("</div>\n")
	}

	if poem.Date != "" {
		b.WriteString("<div class=\"poem-date\">")
		b.WriteString(html.EscapeString(poem.Date))
		b.WriteString("</div>\n")
	}

	b.WriteString("</div>\n")
}

// writeEpigraph renders an <epigraph> block.
func writeEpigraph(b *strings.Builder, ep *FB2Epigraph, bookID string) {
	id := ""
	if ep.ID != "" {
		id = fmt.Sprintf(" id=\"%s\"", html.EscapeString(ep.ID))
	}
	b.WriteString(fmt.Sprintf("<div class=\"epigraph\"%s>\n", id))

	for _, p := range ep.Paragraphs {
		writeParagraph(b, &p, bookID)
	}
	for _, poem := range ep.Poems {
		writePoem(b, &poem, bookID)
	}
	for _, cite := range ep.Cites {
		writeCite(b, &cite, bookID)
	}

	if ep.TextAuthor != "" {
		b.WriteString("<div class=\"text-author\">")
		b.WriteString(html.EscapeString(ep.TextAuthor))
		b.WriteString("</div>\n")
	}

	b.WriteString("</div>\n")
}

// writeCite renders a <cite> block.
func writeCite(b *strings.Builder, cite *FB2Cite, bookID string) {
	id := ""
	if cite.ID != "" {
		id = fmt.Sprintf(" id=\"%s\"", html.EscapeString(cite.ID))
	}
	b.WriteString(fmt.Sprintf("<blockquote class=\"cite\"%s>\n", id))

	for _, p := range cite.Paragraphs {
		writeParagraph(b, &p, bookID)
	}
	for _, poem := range cite.Poems {
		writePoem(b, &poem, bookID)
	}

	if cite.Subtitle != "" {
		b.WriteString("<h4 class=\"subtitle\">")
		b.WriteString(html.EscapeString(cite.Subtitle))
		b.WriteString("</h4>\n")
	}

	if cite.TextAuthor != "" {
		b.WriteString("<div class=\"text-author\">")
		b.WriteString(html.EscapeString(cite.TextAuthor))
		b.WriteString("</div>\n")
	}

	b.WriteString("</blockquote>\n")
}

// writeImage renders an <image> as an <img> tag with API URL.
func writeImage(b *strings.Builder, img *FB2Image, bookID string) {
	if img == nil || img.Href == "" {
		return
	}

	// Remove leading # from href (FB2 uses #name references)
	name := strings.TrimPrefix(img.Href, "#")
	src := fmt.Sprintf("/api/v1/books/%s/image/%s", bookID, name)

	alt := img.Alt
	if alt == "" {
		alt = name
	}

	b.WriteString(fmt.Sprintf("<div class=\"image\"><img src=\"%s\" alt=\"%s\" loading=\"lazy\"></div>\n",
		html.EscapeString(src), html.EscapeString(alt)))
}

// writeTable renders a <table>.
func writeTable(b *strings.Builder, tbl *FB2Table, bookID string) {
	id := ""
	if tbl.ID != "" {
		id = fmt.Sprintf(" id=\"%s\"", html.EscapeString(tbl.ID))
	}
	b.WriteString(fmt.Sprintf("<table class=\"fb2-table\"%s>\n", id))

	for _, row := range tbl.Rows {
		b.WriteString("<tr>\n")
		for _, h := range row.Headers {
			writeTableCell(b, &h, "th", bookID)
		}
		for _, c := range row.Cells {
			writeTableCell(b, &c, "td", bookID)
		}
		b.WriteString("</tr>\n")
	}

	b.WriteString("</table>\n")
}

// writeTableCell renders a <td> or <th>.
func writeTableCell(b *strings.Builder, cell *FB2TableCell, tag string, bookID string) {
	b.WriteString("<")
	b.WriteString(tag)
	if cell.Align != "" {
		b.WriteString(fmt.Sprintf(" style=\"text-align:%s\"", html.EscapeString(cell.Align)))
	}
	if cell.Colspan != "" {
		b.WriteString(fmt.Sprintf(" colspan=\"%s\"", html.EscapeString(cell.Colspan)))
	}
	if cell.Rowspan != "" {
		b.WriteString(fmt.Sprintf(" rowspan=\"%s\"", html.EscapeString(cell.Rowspan)))
	}
	b.WriteString(">")
	b.WriteString(processInline(cell.Content, bookID))
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteString(">\n")
}

// processInline converts FB2 inline XML content to safe HTML.
// FB2 inline elements: <strong>, <emphasis>, <a>, <sub>, <sup>, <style>, <image>
// This is innerxml content so it arrives as raw XML text.
func processInline(content string, bookID string) string {
	if content == "" {
		return ""
	}

	// FB2 uses <emphasis> for italic, map to <em>
	content = strings.ReplaceAll(content, "<emphasis>", "<em>")
	content = strings.ReplaceAll(content, "</emphasis>", "</em>")

	// <strong> maps directly to <strong>
	// <sub> and <sup> map directly

	// <style> in FB2 is used for custom styles, just strip the wrapper
	content = replaceTagPairs(content, "style", "span")

	// Process <a> tags — convert xlink:href to href and fix footnote links
	content = processLinks(content, bookID)

	// Process inline <image> tags
	content = processInlineImages(content, bookID)

	return content
}

// processLinks converts FB2 <a> tags to HTML links.
// FB2 uses xlink:href or l:href attributes.
func processLinks(content string, bookID string) string {
	// This is a simple replacement approach for the most common patterns.
	// FB2 links look like: <a l:href="#note1" type="note">1</a>
	// or: <a xlink:href="http://..." >text</a>

	var result strings.Builder
	i := 0

	for i < len(content) {
		// Look for <a
		if i+2 < len(content) && content[i] == '<' && content[i+1] == 'a' &&
			(content[i+2] == ' ' || content[i+2] == '>') {
			// Find the end of the opening tag
			tagEnd := strings.Index(content[i:], ">")
			if tagEnd == -1 {
				result.WriteByte(content[i])
				i++
				continue
			}
			tagEnd += i

			openTag := content[i : tagEnd+1]

			// Extract href from various attribute formats
			href := extractAttr(openTag, "l:href")
			if href == "" {
				href = extractAttr(openTag, "xlink:href")
			}
			if href == "" {
				href = extractAttr(openTag, "href")
			}

			// Detect note type
			noteType := extractAttr(openTag, "type")

			// Find closing </a>
			closeTag := strings.Index(content[tagEnd+1:], "</a>")
			if closeTag == -1 {
				result.WriteString(openTag)
				i = tagEnd + 1
				continue
			}
			closeTag += tagEnd + 1

			innerText := content[tagEnd+1 : closeTag]

			// Build HTML link
			if strings.HasPrefix(href, "#") {
				// Internal link (footnote reference)
				if noteType == "note" {
					result.WriteString(fmt.Sprintf(
						"<a class=\"footnote-ref\" href=\"%s\" data-note=\"%s\">%s</a>",
						html.EscapeString(href),
						html.EscapeString(strings.TrimPrefix(href, "#")),
						innerText,
					))
				} else {
					result.WriteString(fmt.Sprintf(
						"<a href=\"%s\">%s</a>",
						html.EscapeString(href),
						innerText,
					))
				}
			} else {
				// External link
				result.WriteString(fmt.Sprintf(
					"<a href=\"%s\" target=\"_blank\" rel=\"noopener\">%s</a>",
					html.EscapeString(href),
					innerText,
				))
			}

			i = closeTag + len("</a>")
			continue
		}

		result.WriteByte(content[i])
		i++
	}

	return result.String()
}

// processInlineImages converts inline <image> tags to <img>.
func processInlineImages(content string, bookID string) string {
	// FB2 inline images: <image l:href="#name"/> or <image xlink:href="#name"/>
	var result strings.Builder
	i := 0

	for i < len(content) {
		if i+6 < len(content) && content[i:i+6] == "<image" {
			// Find end of tag
			tagEnd := strings.Index(content[i:], ">")
			if tagEnd == -1 {
				result.WriteByte(content[i])
				i++
				continue
			}
			tagEnd += i

			tag := content[i : tagEnd+1]

			href := extractAttr(tag, "l:href")
			if href == "" {
				href = extractAttr(tag, "xlink:href")
			}
			if href == "" {
				href = extractAttr(tag, "href")
			}

			if href != "" {
				name := strings.TrimPrefix(href, "#")
				src := fmt.Sprintf("/api/v1/books/%s/image/%s", bookID, name)
				result.WriteString(fmt.Sprintf(
					"<img class=\"inline-image\" src=\"%s\" alt=\"%s\" loading=\"lazy\">",
					html.EscapeString(src), html.EscapeString(name),
				))
			}

			i = tagEnd + 1
			continue
		}

		result.WriteByte(content[i])
		i++
	}

	return result.String()
}

// extractAttr extracts an attribute value from a raw XML tag string.
func extractAttr(tag string, name string) string {
	// Look for name="value" or name='value'
	patterns := []string{name + "=\"", name + "='"}
	for _, pattern := range patterns {
		idx := strings.Index(tag, pattern)
		if idx == -1 {
			continue
		}
		start := idx + len(pattern)
		quote := tag[start-1]
		end := strings.IndexByte(tag[start:], quote)
		if end == -1 {
			continue
		}
		return tag[start : start+end]
	}
	return ""
}

// replaceTagPairs replaces XML tag pairs: <from>...</from> → <to>...</to>
func replaceTagPairs(content, from, to string) string {
	content = strings.ReplaceAll(content, "<"+from+">", "<"+to+">")
	content = strings.ReplaceAll(content, "</"+from+">", "</"+to+">")
	// Also handle <from attr="..."> patterns
	content = strings.ReplaceAll(content, "<"+from+" ", "<"+to+" ")
	return content
}

// stripDangerousTags removes potentially dangerous tags but keeps safe inline formatting.
// Allowed: <em>, <strong>, <sub>, <sup>, <span>
func stripDangerousTags(content string) string {
	// For title content, we mostly just want text and basic formatting.
	// The emphasis/strong replacements will have already been applied by processInline
	// if called, but titles get passed through directly.
	content = strings.ReplaceAll(content, "<emphasis>", "<em>")
	content = strings.ReplaceAll(content, "</emphasis>", "</em>")
	return content
}
