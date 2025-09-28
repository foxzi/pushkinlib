package opds

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"strings"
)

// LoadGenreNames loads genre code translations from CSV file.
// The CSV is expected to have headers with at least "code" and "name_ru" columns.
// Returns a map of lowercased genre codes to localized names.
func LoadGenreNames(path string) (map[string]string, error) {
	genres := make(map[string]string)
	if strings.TrimSpace(path) == "" {
		return genres, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return genres, nil
		}
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return genres, nil
		}
		return nil, err
	}

	for i := range headers {
		headers[i] = strings.TrimSpace(strings.ToLower(headers[i]))
	}

	codeIndex := indexOf(headers, "code")
	nameIndex := indexOf(headers, "name_ru")
	if nameIndex == -1 {
		nameIndex = indexOf(headers, "name")
	}

	if codeIndex == -1 || nameIndex == -1 {
		return genres, nil
	}

	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(record) <= codeIndex {
			continue
		}

		code := strings.TrimSpace(strings.ToLower(record[codeIndex]))
		if code == "" {
			continue
		}

		name := code
		if len(record) > nameIndex {
			value := strings.TrimSpace(record[nameIndex])
			if value != "" {
				name = value
			}
		}

		genres[code] = name
	}

	return genres, nil
}

func indexOf(slice []string, target string) int {
	for i, v := range slice {
		if v == target {
			return i
		}
	}
	return -1
}

// genreLabel returns a human-friendly label for a genre code.
func (b *Builder) genreLabel(code string) string {
	codes := splitGenreCodes(code)
	if len(codes) == 0 {
		return code
	}

	labels := make([]string, 0, len(codes))
	seen := make(map[string]struct{})

	for _, raw := range codes {
		if raw == "" {
			continue
		}

		normalized := strings.TrimSpace(strings.ToLower(raw))
		label := strings.TrimSpace(raw)
		if mapped, ok := b.genreNames[normalized]; ok && mapped != "" {
			label = mapped
		}

		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
	}

	if len(labels) == 0 {
		return code
	}

	if len(labels) == 1 {
		return labels[0]
	}

	return strings.Join(labels, ", ")
}

func splitGenreCodes(code string) []string {
	return strings.FieldsFunc(code, func(r rune) bool {
		switch r {
		case ':', ',', ';', '|':
			return true
		default:
			return false
		}
	})
}
