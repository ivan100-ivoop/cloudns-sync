package bind

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseMasterZones(t *testing.T) {
	contents := []byte(`
// comments and whitespace should not affect discovery
zone "Example.COM." IN { type master; file "one"; };
zone 'example.net' {
    type
        master;
    file "two";
};
zone "secondary.example" { type slave; masters { 192.0.2.10; }; };
zone "Example.COM" { type master; file "duplicate"; };
`)
	got, err := ParseMasterZones(contents)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "example.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseMasterZones() = %#v, want %#v", got, want)
	}
}

func TestParseMasterZonesMalformed(t *testing.T) {
	for name, contents := range map[string]string{
		"unterminated zone":  `zone "example.com" { type master;`,
		"unterminated quote": `zone "example.com { type master; };`,
		"include":            `include "/etc/named.local";`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMasterZones([]byte(contents)); err == nil {
				t.Fatal("expected parsing error")
			}
		})
	}
}

func TestParseMasterZonesEmpty(t *testing.T) {
	got, err := ParseMasterZones([]byte("# no zones\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no zones, got %#v", got)
	}
}

func TestParseMasterZonesFileExpandsIncludesAndBlockComments(t *testing.T) {
	directory := t.TempDir()
	includeDirectory := filepath.Join(directory, "zones")
	if err := os.Mkdir(includeDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(directory, "named.conf")
	includePath := filepath.Join(includeDirectory, "local.conf")
	if err := os.WriteFile(mainPath, []byte(`/* named.conf comment */
include "zones/local.conf";
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte(`zone "included.example" {
    type master;
    file "/var/named/included.example.db";
};
`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ParseMasterZonesFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"included.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseMasterZonesFile() = %#v, want %#v", got, want)
	}
}
