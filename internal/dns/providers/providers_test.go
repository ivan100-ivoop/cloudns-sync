package providers

import (
	"testing"

	"github.com/ivan100-ivoop/cloudns-sync/internal/config"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns/bind"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns/powerdns"
)

func TestNewNamedProvider(t *testing.T) {
	source, path, err := New(Options{Settings: config.ProviderConfig{
		Provider: "named", Type: "bind", ConfigFile: "./testdata/named.conf",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if path != "./testdata/named.conf" {
		t.Fatalf("path = %q", path)
	}
	if _, ok := source.(bind.Source); !ok {
		t.Fatalf("source type = %T, want bind.Source", source)
	}
}

func TestNewPowerDNSProvider(t *testing.T) {
	source, _, err := New(Options{Settings: config.ProviderConfig{
		Provider: "powerdns", Type: "command", Command: "pdnsutil", Args: []string{"list-all-zones", "--no-color"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := source.(powerdns.Source); !ok {
		t.Fatalf("source type = %T, want powerdns.Source", source)
	}
}
