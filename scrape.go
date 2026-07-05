package hayden

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// Find renders the target URL with headless chrome and reports whether the
// search string is present.
func (cfg *Config) Find(ctx context.Context, target *url.URL, search string) (bool, error) {
	cctx, ccancel := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(cfg.Log.Infof),
		chromedp.WithDebugf(cfg.Log.Debugf),
		chromedp.WithErrorf(cfg.Log.Errorf),
	)
	defer ccancel()

	tctx, tcancel := context.WithTimeout(cctx, 150*time.Second)
	defer tcancel()

	var htmlContent string
	if err := chromedp.Run(
		tctx,
		chromedp.Navigate(target.String()),
		chromedp.InnerHTML(`body`, &htmlContent, chromedp.ByJSPath),
	); err != nil {
		return false, fmt.Errorf("chrome error: %w", err)
	}

	return cfg.scanHTMLContent(ctx, htmlContent, search)
}

//nolint:unparam // stub: real matching (and a non-constant result) lands in the matching phase
func (cfg *Config) scanHTMLContent(_ context.Context, html string, _ string) (bool, error) {
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false, err
	}

	cfg.Log.Debugw("all text", "text", dom.Text())

	return false, fmt.Errorf("not implemented")
}
