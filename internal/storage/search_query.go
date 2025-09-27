package storage

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	searchFieldRegex     = regexp.MustCompile(`(?i)\b(author|authors|автор|авторы|series|серия|серии|title|название|annotation|описание|description):("([^"\\]|\\.)*"|\S+)`)
	ftsSearchableColumns = []string{"title", "annotation", "authors", "series"}
)

type structuredQuery struct {
	Remainder       string
	GeneralTerms    []string
	TitleTerms      []string
	AuthorTerms     []string
	SeriesTerms     []string
	AnnotationTerms []string
}

func prepareFTSSearch(input string) (string, string) {
	parsed := parseSearchQuery(input)
	ftsExpr := buildFTSExpression(parsed)

	fallback := normalizeWhitespace(parsed.Remainder)
	if fallback == "" && len(parsed.GeneralTerms) > 0 {
		fallback = strings.Join(uniqueTokens(parsed.GeneralTerms), " ")
	}

	return ftsExpr, fallback
}

func parseSearchQuery(input string) structuredQuery {
	result := structuredQuery{}
	if strings.TrimSpace(input) == "" {
		return result
	}

	matches := searchFieldRegex.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		result.Remainder = input
		result.GeneralTerms = tokenizeText(input)
		return result
	}

	var remainder strings.Builder
	last := 0

	for _, idx := range matches {
		start := idx[0]
		end := idx[1]
		fieldStart := idx[2]
		fieldEnd := idx[3]
		valueStart := idx[4]
		valueEnd := idx[5]

		remainder.WriteString(input[last:start])

		rawField := input[fieldStart:fieldEnd]
		normalizedField := normalizeSearchField(rawField)
		rawValue := input[valueStart:valueEnd]

		if normalizedField == "" {
			remainder.WriteString(input[start:end])
			last = end
			continue
		}

		value := unquoteSearchValue(rawValue)
		tokens := tokenizeText(value)

		switch normalizedField {
		case "title":
			result.TitleTerms = append(result.TitleTerms, tokens...)
		case "authors":
			result.AuthorTerms = append(result.AuthorTerms, tokens...)
		case "series":
			result.SeriesTerms = append(result.SeriesTerms, tokens...)
		case "annotation":
			result.AnnotationTerms = append(result.AnnotationTerms, tokens...)
		}

		last = end
	}

	remainder.WriteString(input[last:])
	result.Remainder = remainder.String()
	result.GeneralTerms = tokenizeText(result.Remainder)

	return result
}

func buildFTSExpression(q structuredQuery) string {
	var clauses []string

	if clause := buildGeneralFTSClause(q.GeneralTerms); clause != "" {
		clauses = append(clauses, clause)
	}

	if clause := buildFieldFTSClause("title", q.TitleTerms); clause != "" {
		clauses = append(clauses, clause)
	}

	if clause := buildFieldFTSClause("authors", q.AuthorTerms); clause != "" {
		clauses = append(clauses, clause)
	}

	if clause := buildFieldFTSClause("series", q.SeriesTerms); clause != "" {
		clauses = append(clauses, clause)
	}

	if clause := buildFieldFTSClause("annotation", q.AnnotationTerms); clause != "" {
		clauses = append(clauses, clause)
	}

	switch len(clauses) {
	case 0:
		return ""
	case 1:
		return clauses[0]
	default:
		return strings.Join(clauses, " AND ")
	}
}

func buildGeneralFTSClause(tokens []string) string {
	unique := uniqueTokens(tokens)
	if len(unique) == 0 {
		return ""
	}

	perToken := make([]string, 0, len(unique))
	for _, token := range unique {
		formatted := formatFTSToken(token)
		columnClauses := make([]string, 0, len(ftsSearchableColumns))
		for _, column := range ftsSearchableColumns {
			columnClauses = append(columnClauses, column+":"+formatted)
		}
		perToken = append(perToken, "("+strings.Join(columnClauses, " OR ")+")")
	}

	if len(perToken) == 1 {
		return perToken[0]
	}

	return strings.Join(perToken, " AND ")
}

func buildFieldFTSClause(field string, tokens []string) string {
	unique := uniqueTokens(tokens)
	if len(unique) == 0 {
		return ""
	}

	parts := make([]string, 0, len(unique))
	for _, token := range unique {
		parts = append(parts, field+":"+formatFTSToken(token))
	}

	if len(parts) == 1 {
		return parts[0]
	}

	return strings.Join(parts, " AND ")
}

func normalizeSearchField(field string) string {
	switch strings.ToLower(field) {
	case "author", "authors", "автор", "авторы":
		return "authors"
	case "series", "серия", "серии":
		return "series"
	case "title", "название":
		return "title"
	case "annotation", "описание", "description":
		return "annotation"
	default:
		return ""
	}
}

func unquoteSearchValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		content := trimmed[1 : len(trimmed)-1]
		var builder strings.Builder
		escaped := false
		for _, r := range content {
			if escaped {
				builder.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			builder.WriteRune(r)
		}
		if escaped {
			builder.WriteRune('\\')
		}
		return builder.String()
	}
	return trimmed
}

func tokenizeText(input string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
			continue
		}

		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func uniqueTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(tokens))
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func formatFTSToken(token string) string {
	if token == "" {
		return ""
	}
	if strings.HasSuffix(token, "*") {
		return token
	}
	return token + "*"
}

func normalizeWhitespace(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	prevSpace := false
	for _, r := range trimmed {
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			builder.WriteRune(' ')
			prevSpace = true
			continue
		}

		builder.WriteRune(r)
		prevSpace = false
	}

	return builder.String()
}
