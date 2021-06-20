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
	// create chrome instance
	cctx, ccancel := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	tctx, tcancel = context.WithTimeout(cctx, 15*time.Second)
	defer tcancel()

	if err := chromedp.Run(
		cctx,
		emulation.SetUserAgentOverride("Chrome Hayden"),
		chromedp.Navigate(`https://github.com`),
		chromedp.ScrollIntoView(`footer`),
		chromedp.WaitVisible(`footer > div`),
		chromedp.Text(`h1`, &res, chromedp.NodeVisible, chromedp.ByQuery),
	); err != nil {
		return false, fmt.Errorf("chrome error: %w", err)
	}

}
