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

func (cfg *Config) Find(ctx context.Context, target url.URL, search string) (bool, error) {
	cctx, ccancel := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(cfg.Log.Debugf),
	)
	defer ccancel()

	tctx, tcancel := context.WithTimeout(cctx, 15*time.Second)
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

	return scanHTMLContent(ctx, htmlContent, search)
}

func scanHTMLContent(ctx context.Context, html string, search string) (bool, error) {
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false, err
	}

	return false, fmt.Errorf("not implemented")
}
