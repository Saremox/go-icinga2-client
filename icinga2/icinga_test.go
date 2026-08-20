package icinga2

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestWebClientConcurrentURLAccess is a regression test for a data race on
// WebClient.URL: SetIcingaUrl (used by callers failing over between Icinga
// instances) could be called concurrently with any request-issuing method
// (TestIcingaApi, GetClientConfig, GetHost, ...), all of which read the URL
// to build the request. Run with -race to catch a regression.
func TestWebClientConcurrentURLAccess(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	})

	srv1 := httptest.NewServer(okHandler)
	defer srv1.Close()
	srv2 := httptest.NewServer(okHandler)
	defer srv2.Close()

	client, err := New(&WebClient{URL: srv1.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				client.SetIcingaUrl(srv1.URL)
			} else {
				client.SetIcingaUrl(srv2.URL)
			}
		}()
		go func() {
			defer wg.Done()
			_ = client.TestIcingaApi()
			_ = client.GetClientConfig()
		}()
	}
	wg.Wait()
}
