package bind

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var zoneStart = regexp.MustCompile(`(?i)\bzone\s+(?:"([^"]+)"|'([^']+)')(?:\s+[a-z0-9_-]+)?\s*\{`)
var includeDirective = regexp.MustCompile(`(?im)^\s*include\s+`)

// ParseMasterZones extracts only zones whose declaration has type master.
func ParseMasterZones(contents []byte) ([]string, error) {
	cleaned, err := stripComments(string(contents))
	if err != nil {
		return nil, err
	}
	if includeDirective.MatchString(cleaned) {
		return nil, fmt.Errorf("include directives are not supported; provide a self-contained configuration")
	}
	zones := make(map[string]struct{})
	position := 0
	for position < len(cleaned) {
		position += len(cleaned[position:]) - len(strings.TrimLeft(cleaned[position:], " \t\r\n;"))
		match := zoneStart.FindStringSubmatchIndex(cleaned[position:])
		if match == nil {
			break
		}
		name := ""
		if match[2] >= 0 {
			name = cleaned[position+match[2] : position+match[3]]
		} else {
			name = cleaned[position+match[4] : position+match[5]]
		}
		name = strings.TrimSpace(name)
		open := position + match[1] - 1
		close, err := matchingBrace(cleaned, open)
		if err != nil {
			return nil, fmt.Errorf("zone %q: %w", name, err)
		}
		body := cleaned[open+1 : close]
		if zoneType(body) == "master" {
			zones[normalizeZone(name)] = struct{}{}
		}
		position = close + 1
	}
	result := make([]string, 0, len(zones))
	for zone := range zones {
		if zone != "" {
			result = append(result, zone)
		}
	}
	sort.Strings(result)
	return result, nil
}

func zoneType(body string) string {
	fields := strings.Fields(strings.ToLower(body))
	for index, field := range fields {
		if field == "type" && index+1 < len(fields) {
			return strings.Trim(fields[index+1], ";")
		}
	}
	return ""
}

func normalizeZone(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func matchingBrace(contents string, open int) (int, error) {
	depth := 0
	for index := open; index < len(contents); index++ {
		switch contents[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated block")
}

func stripComments(contents string) (string, error) {
	var builder strings.Builder
	inQuote := byte(0)
	escaped := false
	inBlockComment := false
	for index := 0; index < len(contents); index++ {
		char := contents[index]
		if inBlockComment {
			if char == '*' && index+1 < len(contents) && contents[index+1] == '/' {
				inBlockComment = false
				index++
				builder.WriteByte(' ')
			} else if char == '\n' {
				builder.WriteByte('\n')
			}
			continue
		}
		if inQuote != 0 {
			builder.WriteByte(char)
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == inQuote {
				inQuote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			inQuote = char
			builder.WriteByte(char)
			continue
		}
		if char == '/' && index+1 < len(contents) && contents[index+1] == '*' {
			inBlockComment = true
			index++
			continue
		}
		if char == '#' || char == ';' && false { // semicolons terminate statements, not comments
			for index < len(contents) && contents[index] != '\n' {
				index++
			}
			builder.WriteByte('\n')
			continue
		}
		if char == '/' && index+1 < len(contents) && contents[index+1] == '/' {
			index++
			for index < len(contents) && contents[index] != '\n' {
				index++
			}
			builder.WriteByte('\n')
			continue
		}
		builder.WriteByte(char)
	}
	if inQuote != 0 {
		return "", fmt.Errorf("unterminated quoted string")
	}
	if inBlockComment {
		return "", fmt.Errorf("unterminated block comment")
	}
	return builder.String(), nil
}

func preview(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 40 {
		return value[:40] + "..."
	}
	return value
}
