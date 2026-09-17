package bind

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ivan100-ivoop/cloudns-sync/internal/dns"
)

type Source struct{ Path string }

var _ dns.ZoneSource = Source{}

func (source Source) ListZones(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	zones, err := ParseMasterZonesFile(source.Path)
	if err != nil {
		return nil, fmt.Errorf("parse BIND configuration %q: %w", source.Path, err)
	}
	return zones, nil
}

// ParseMasterZonesFile expands BIND include directives before parsing zones.
func ParseMasterZonesFile(path string) ([]string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve BIND configuration %q: %w", path, err)
	}
	contents, err := expandIncludes(absolutePath, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	return ParseMasterZones([]byte(contents))
}

var includeFile = regexp.MustCompile(`(?im)^\s*include\s+"([^"]+)"\s*;\s*`)

func expandIncludes(path string, visiting map[string]bool) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve include %q: %w", path, err)
	}
	if visiting[path] {
		return "", fmt.Errorf("cyclic BIND include detected at %q", path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read BIND include %q: %w", path, err)
	}
	cleaned, err := stripComments(string(contents))
	if err != nil {
		return "", fmt.Errorf("strip comments from BIND file %q: %w", path, err)
	}
	visiting[path] = true
	defer delete(visiting, path)

	matches := includeFile.FindAllStringSubmatchIndex(cleaned, -1)
	if len(matches) == 0 {
		return cleaned, nil
	}
	var builder strings.Builder
	position := 0
	for _, match := range matches {
		builder.WriteString(cleaned[position:match[0]])
		pattern := cleaned[match[2]:match[3]]
		includePath := pattern
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(filepath.Dir(path), includePath)
		}
		paths, err := filepath.Glob(includePath)
		if err != nil {
			return "", fmt.Errorf("invalid BIND include pattern %q in %q: %w", pattern, path, err)
		}
		if len(paths) == 0 {
			return "", fmt.Errorf("BIND include %q from %q matched no files", pattern, path)
		}
		sort.Strings(paths)
		for _, includedPath := range paths {
			included, err := expandIncludes(includedPath, visiting)
			if err != nil {
				return "", err
			}
			builder.WriteString("\n")
			builder.WriteString(included)
		}
		position = match[1]
	}
	builder.WriteString(cleaned[position:])
	return builder.String(), nil
}
