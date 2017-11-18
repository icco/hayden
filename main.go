package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/fatih/set.v0"
)

func getLinks(uri *url.URL) []string {
	s := set.New()
	s.Add(uri.String())

	doc, err := goquery.NewDocument(uri.String())
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body a").Each(func(index int, item *goquery.Selection) {
		linkTag := item
		link, _ := linkTag.Attr("href")
		log.Printf("%d: %s", index, link)

		parsedUri, err := url.Parse(link)
		if err != nil {
			log.Fatal(err)
		}
		s.Add(parsedUri.String())
	})

	return set.StringSlice(s)
}

func main() {
	u, err := url.Parse("https://theintercept.com")
	if err != nil {
		log.Fatal(err)
	}

	urls := getLinks(u)
	for _, v := range urls {
		fmt.Println(v)
	}
}
