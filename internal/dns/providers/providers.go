package providers

import (
	"fmt"
	"strings"

	"github.com/ivan100-ivoop/cloudns-sync/internal/config"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns/bind"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns/powerdns"
)

type Options struct {
	Name       string
	ConfigFile string
	Command    string
	Args       []string
	Settings   config.ProviderConfig
}

// New builds the selected provider while keeping provider-specific parsing
// outside the CLI and future synchronization code.
func New(options Options) (dns.ZoneSource, string, error) {
	name := strings.ToLower(strings.TrimSpace(options.Name))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(options.Settings.Provider))
	}
	if name == "" {
		name = "named"
	}
	settings := options.Settings

	switch name {
	case "named", "bind":
		path := firstNonEmpty(options.ConfigFile, settings.ConfigFile)
		if path == "" {
			return nil, "", fmt.Errorf("provider %q requires config_file", name)
		}
		return bind.Source{Path: path}, path, nil
	case "powerdns":
		command := firstNonEmpty(options.Command, settings.Command, "pdnsutil")
		args := options.Args
		if len(args) == 0 {
			args = settings.Args
		}
		if len(args) == 0 {
			args = []string{"list-all-zones"}
		}
		return powerdns.Source{Command: command, Args: args}, command, nil
	default:
		return nil, "", fmt.Errorf("unsupported DNS provider %q", name)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
