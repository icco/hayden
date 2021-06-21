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

func (cfg *Config) Find(ctx context.Context, target *url.URL, search string) (bool, error) {
	cctx, ccancel := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(cfg.Log.Debugf),
	)
	defer ccancel()

	tctx, tcancel := context.WithTimeout(cctx, 150*time.Second)
	defer tcancel()

	var htmlContent string
	if err := chromedp.Run(
		tctx,
		chromedp.Navigate(target.String()),
		chromedp.WaitVisible(`body`),
		chromedp.InnerHTML(`body`, &htmlContent, chromedp.ByJSPath),
	); err != nil {
		return false, fmt.Errorf("chrome error: %w", err)
	}

	return cfg.scanHTMLContent(ctx, htmlContent, search)
}

func (cfg *Config) scanHTMLContent(ctx context.Context, html string, search string) (bool, error) {
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false, err
	}

	cfg.Log.Debugw("all text", "text", dom.Text())

	return false, fmt.Errorf("not implemented")
}
