package config

import (
	"strings"
	"testing"
	"time"
)

const validConfig = `settings:
  master_ip: ["192.0.2.10", "2001:db8::10"]
  sync_interval: 5s
logging:
  directory: ./logs
  level: info
  error_file: error-{date}.log
`

func TestParseAndValidate(t *testing.T) {
	cfg, err := Parse([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Settings.MasterIP) != 2 || cfg.Settings.SyncInterval.Duration() != 5*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestInvalidSyncInterval(t *testing.T) {
	_, err := Parse([]byte(strings.Replace(validConfig, "5s", "500ms", 1)))
	if err == nil {
		t.Fatal("expected interval validation error")
	}
}

func TestInvalidMasterIP(t *testing.T) {
	_, err := Parse([]byte(strings.Replace(validConfig, "192.0.2.10", "not-an-ip", 1)))
	if err == nil {
		t.Fatal("expected IP validation error")
	}
}

func TestInvalidDNSBackend(t *testing.T) {
	_, err := Parse([]byte(strings.Replace(validConfig, "logging:", "  dns_provider: unknown\nlogging:", 1)))
	if err == nil {
		t.Fatal("expected DNS backend validation error")
	}
}

func TestPowerDNSBackendRequiresCommand(t *testing.T) {
	contents := strings.Replace(validConfig, "logging:", "  dns_provider: powerdns\nlogging:", 1)
	if _, err := Parse([]byte(contents)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadProvider(t *testing.T) {
	provider, err := LoadProvider("../../configs/providers/named.yml")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Provider != "named" || provider.Type != "bind" {
		t.Fatalf("unexpected provider config: %+v", provider)
	}
	if provider.ConfigFile != "./testdata/named.conf" {
		t.Fatalf("config file = %q", provider.ConfigFile)
	}
}

func TestLoadProvidersUsesNamesFromFiles(t *testing.T) {
	providers, err := LoadProviders("../../configs/providers")
	if err != nil {
		t.Fatal(err)
	}
	if providers["named"].Type != "bind" || providers["powerdns"].Type != "command" {
		t.Fatalf("unexpected providers: %+v", providers)
	}
}
