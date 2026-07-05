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

func TestShouldNotifyOnce(t *testing.T) {
	cases := []struct {
		lastMatched, matched, want bool
	}{
		{false, true, true},   // transition into match → notify
		{true, true, false},   // still matching → no repeat
		{true, false, false},  // no longer matching → no notify
		{false, false, false}, // never matched
	}
	for _, c := range cases {
		tg := &Target{LastMatched: c.lastMatched}
		if got := ShouldNotify(tg, c.matched); got != c.want {
			t.Errorf("ShouldNotify(last=%v, matched=%v) = %v, want %v", c.lastMatched, c.matched, got, c.want)
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
