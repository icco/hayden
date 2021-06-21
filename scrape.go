package hayden

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func Find(ctx context.Context, target url.URL, searchRegexp string) (bool, error) {
	cctx, ccancel := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(log.Printf),
	)
	defer ccancel()

	tctx, tcancel := context.WithTimeout(cctx, 15*time.Second)
	defer tcancel()

	var res string
	if err := chromedp.Run(
		tctx,
		emulation.SetUserAgentOverride("Chrome Hayden"),
		chromedp.Navigate(`https://github.com`),
		chromedp.ScrollIntoView(`footer`),
		chromedp.WaitVisible(`footer > div`),
		chromedp.Text(`h1`, &res, chromedp.NodeVisible, chromedp.ByQuery),
	); err != nil {
		return false, fmt.Errorf("chrome error: %w", err)
	}

	return false, nil
}
