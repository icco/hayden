package hayden

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chromedp/chromedp"
)

// Fetcher retrieves the content of a target for matching.
type Fetcher interface {
	Fetch(ctx context.Context, target *Target) ([]byte, error)
}

// httpFetcher does a plain HTTP GET. Best for APIs, JSON, and static pages.
type httpFetcher struct{ Client *http.Client }

func (f httpFetcher) Fetch(ctx context.Context, target *Target) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", target.URL, err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", target.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("fetching %s: status %d", target.URL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body of %s: %w", target.URL, err)
	}

	return body, nil
}

// headlessFetcher renders the page with headless chrome, for JS-heavy pages.
type headlessFetcher struct{}

func (headlessFetcher) Fetch(ctx context.Context, target *Target) ([]byte, error) {
	cctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	tctx, tcancel := context.WithTimeout(cctx, 150*time.Second)
	defer tcancel()

	var html string
	if err := chromedp.Run(tctx,
		chromedp.Navigate(target.URL),
		chromedp.InnerHTML("body", &html, chromedp.ByJSPath),
	); err != nil {
		return nil, fmt.Errorf("headless fetch %s: %w", target.URL, err)
	}

	return []byte(html), nil
}

// FetcherFor returns the Fetcher for a fetch mode.
func FetcherFor(fetchMode string) (Fetcher, error) {
	switch fetchMode {
	case "http", "":
		return httpFetcher{Client: &http.Client{Timeout: 30 * time.Second}}, nil
	case "headless":
		return headlessFetcher{}, nil
	default:
		return nil, fmt.Errorf("unsupported fetch mode %q", fetchMode)
	}
}
