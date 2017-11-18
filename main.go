package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

func linkScrape(uri *url.URL) {
	doc, err := goquery.NewDocument(uri.String())
	if err != nil {
		log.Fatal(err)
	}

	// use CSS selector found with the browser inspector
	// for each, use index and item
	doc.Find("body a").Each(func(index int, item *goquery.Selection) {
		linkTag := item
		link, _ := linkTag.Attr("href")
		linkText := linkTag.Text()
		fmt.Printf("Link #%d: '%s' - '%s'\n", index, linkText, link)
	})
}

func main() {
	u, err := url.Parse("https://natwelch.com")
	if err != nil {
		log.Fatal(err)
	}

	linkScrape(u)
}
