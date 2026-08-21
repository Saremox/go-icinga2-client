package icinga2

import (
	"net/http"
	"testing"
)

// TestRedactAuthorization is a regression test for a bug where debug
// request logging (session.logRequest, enabled via WebClient.Debug) wrote
// the full request header - including any Authorization value, e.g. HTTP
// Basic Auth credentials - to the log verbatim.
func TestRedactAuthorization(t *testing.T) {
	t.Run("redacts a present Authorization value without mutating the original", func(t *testing.T) {
		header := http.Header{}
		header.Set("Authorization", "Basic dXNlcjpwYXNzd29yZA==")
		header.Set("Accept", "application/json")

		redacted := redactAuthorization(header)

		if got := redacted.Get("Authorization"); got == "Basic dXNlcjpwYXNzd29yZA==" || got == "" {
			t.Fatalf("redacted Authorization = %q, want a redacted placeholder", got)
		}
		if got := header.Get("Authorization"); got != "Basic dXNlcjpwYXNzd29yZA==" {
			t.Fatalf("original header was mutated: Authorization = %q", got)
		}
		if got := redacted.Get("Accept"); got != "application/json" {
			t.Fatalf("unrelated header Accept = %q, want unchanged", got)
		}
	})

	t.Run("leaves a header with no Authorization untouched", func(t *testing.T) {
		header := http.Header{}
		header.Set("Accept", "application/json")

		redacted := redactAuthorization(header)

		if got := redacted.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want unchanged", got)
		}
		if redacted.Get("Authorization") != "" {
			t.Fatalf("Authorization = %q, want empty", redacted.Get("Authorization"))
		}
	})
}
