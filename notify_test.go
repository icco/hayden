package hayden

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHTTPNotifierDiscord(t *testing.T) {
	var got map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Route discord.com to the test server so the real hook URL shape is exercised.
	client := srv.Client()
	client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // test server

	n := HTTPNotifier{Client: client}
	tg := &Target{Name: "m8", Hook: "https://discord.com/api/webhooks/1/secret-token"}
	ev := Event{Target: "m8", URL: "https://shop.example/m8", Matched: true, Match: "@everyone true"}
	if err := n.Notify(context.Background(), tg, ev); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if got["content"] != "m8 matched: @everyone true\nhttps://shop.example/m8" {
		t.Errorf("content = %q", got["content"])
	}
	if am, _ := got["allowed_mentions"].(map[string]any); am == nil {
		t.Errorf("allowed_mentions missing: %v", got)
	}
}

func TestHTTPNotifierRedactsHookInErrors(t *testing.T) {
	n := HTTPNotifier{Client: &http.Client{Timeout: time.Second}}
	err := n.Notify(context.Background(), &Target{Name: "t", Hook: "http://127.0.0.1:1/api/webhooks/1/secret-token"}, Event{})
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("error = %v, want one without the token", err)
	}
}
