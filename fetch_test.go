package hayden

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "hello world")
	}))
	defer srv.Close()

	f, err := FetcherFor("http")
	if err != nil {
		t.Fatalf("FetcherFor: %v", err)
	}
	body, err := f.Fetch(context.Background(), &Target{URL: srv.URL})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "hello world" {
		t.Errorf("body = %q, want %q", body, "hello world")
	}
}

func TestHTTPFetcherErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	f, _ := FetcherFor("http")
	if _, err := f.Fetch(context.Background(), &Target{URL: srv.URL}); err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestFetcherForUnsupported(t *testing.T) {
	if _, err := FetcherFor("carrier-pigeon"); err == nil {
		t.Error("expected error for unsupported fetch mode")
	}
}
