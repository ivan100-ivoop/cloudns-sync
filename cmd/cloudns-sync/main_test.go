package main

import (
	"testing"

	"github.com/ivan100-ivoop/cloudns-sync/internal/cloudns"
)

func TestExcludedRemoteZonesAreIgnored(t *testing.T) {
	existing := map[string]cloudns.Zone{
		"example.com":  {Name: "example.com", Type: "slave"},
		"example.arpa": {Name: "example.arpa", Type: "slave"},
	}
	excluded := []string{"*.arpa"}

	filterExcludedZones(existing, excluded)

	if _, ok := existing["example.arpa"]; ok {
		t.Fatal("excluded remote zone should not be considered")
	}
	if _, ok := existing["example.com"]; !ok {
		t.Fatal("non-excluded remote zone should remain")
	}
}

func TestResolveDeleteMissing(t *testing.T) {
	tests := []struct {
		name                          string
		configured, enabled, disabled bool
		want                          bool
	}{
		{name: "disabled by default"},
		{name: "enabled globally", configured: true, want: true},
		{name: "enabled by command line", enabled: true, want: true},
		{name: "disabled by command line", configured: true, disabled: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveDeleteMissing(test.configured, test.enabled, test.disabled)
			if got != test.want {
				t.Fatalf("resolveDeleteMissing(%t, %t, %t) = %t, want %t", test.configured, test.enabled, test.disabled, got, test.want)
			}
		})
	}
}
