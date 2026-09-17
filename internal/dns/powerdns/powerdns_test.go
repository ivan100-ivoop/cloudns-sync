package powerdns

import (
	"reflect"
	"testing"
)

func TestParseZoneList(t *testing.T) {
	got, err := ParseZoneList([]byte("Example.COM.\nexample.net\n# comment\nexample.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "example.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseZoneList() = %#v, want %#v", got, want)
	}
}

func TestParseZoneListRejectsRecords(t *testing.T) {
	if _, err := ParseZoneList([]byte("example.com 127.0.0.1\n")); err == nil {
		t.Fatal("expected malformed zone list error")
	}
}
