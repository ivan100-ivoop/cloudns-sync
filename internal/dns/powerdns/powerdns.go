package powerdns

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Source discovers zones through the local PowerDNS administration command.
// It does not require access to PowerDNS's database backend.
type Source struct {
	Command string
	Args    []string
}

func (source Source) ListZones(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(source.Command) == "" {
		return nil, fmt.Errorf("PowerDNS command is required")
	}
	output, err := exec.CommandContext(ctx, source.Command, source.Args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run PowerDNS command %q: %w: %s", source.Command, err, strings.TrimSpace(string(output)))
	}
	return ParseZoneList(output)
}

func ParseZoneList(output []byte) ([]string, error) {
	zones := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 1 {
			return nil, fmt.Errorf("invalid PowerDNS zone list at line %d", lineNumber)
		}
		zone := normalizeZone(fields[0])
		if zone == "" || strings.ContainsAny(zone, "\\/\"'") {
			return nil, fmt.Errorf("invalid PowerDNS zone %q at line %d", fields[0], lineNumber)
		}
		zones[zone] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read PowerDNS zone list: %w", err)
	}
	result := make([]string, 0, len(zones))
	for zone := range zones {
		result = append(result, zone)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeZone(zone string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
}
