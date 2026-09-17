package logger

import "testing"

func TestSanitize(t *testing.T) {
	message := "auth_password=secret api_key:abc authorization=BearerXYZ token=tok"
	got := Sanitize(message)
	want := "auth_password=[REDACTED] api_key=[REDACTED] authorization=[REDACTED] token=[REDACTED]"
	if got != want {
		t.Fatalf("Sanitize() = %q, want %q", got, want)
	}
}
