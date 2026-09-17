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
