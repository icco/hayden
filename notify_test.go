package hayden

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShouldNotify(t *testing.T) {
	cases := []struct {
		name        string
		mode        string
		lastMatched bool
		lastHash    string
		matched     bool
		hash        string
		want        bool
	}{
		{"once transition", "once", false, "", true, "h1", true},
		{"once repeat", "once", true, "h1", true, "h1", false},
		{"once no match", "once", true, "h1", false, "h1", false},
		{"change first", "change", false, "", true, "h1", true},
		{"change new hash", "change", true, "h1", true, "h2", true},
		{"change same hash", "change", true, "h1", true, "h1", false},
		{"change no match", "change", true, "h1", false, "h2", false},
	}
	for _, c := range cases {
		tg := &Target{NotifyMode: c.mode, LastMatched: c.lastMatched, LastContentHash: c.lastHash}
		if got := ShouldNotify(tg, c.matched, c.hash); got != c.want {
			t.Errorf("%s: ShouldNotify = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHTTPNotifierFallbackAndPayload(t *testing.T) {
	var gotBody []byte
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := HTTPNotifier{Client: srv.Client(), DefaultHook: srv.URL}
	ev := Event{Target: "t", URL: "https://example.com", Matched: true, MatchedAt: time.Now().UTC(), MatchType: "substring"}

	// Empty target hook → falls back to DefaultHook.
	if err := n.Notify(context.Background(), &Target{Name: "t"}, ev); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	var decoded Event
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if !decoded.Matched || decoded.Target != "t" || decoded.MatchType != "substring" {
		t.Errorf("payload = %+v", decoded)
	}
}

func TestHTTPNotifierNoHook(t *testing.T) {
	n := HTTPNotifier{}
	if err := n.Notify(context.Background(), &Target{Name: "t"}, Event{}); err == nil {
		t.Error("expected error when no hook is configured")
	}
}
