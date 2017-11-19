// A tool and set of functions for archiving links in a page.
package hayden

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/fatih/set.v0"
)

// Given a url, scrapes and grabs all things linked with an anchor tag. It
// returns those in a list of unique strings.
func GetLinks(baseUri *url.URL) []string {
	s := set.New()
	s.Add(baseUri.String())

	doc, err := goquery.NewDocument(baseUri.String())
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body a").Each(func(index int, item *goquery.Selection) {
		linkTag := item
		link, _ := linkTag.Attr("href")
		log.Printf("%d: %s", index, link)

		// Add to final set.
		s.Add(ParseLink(link, baseUri).String())
	})

	return set.StringSlice(s)
}

// Given a uri string, return a nicely filled out URL. If context is provided,
// we'll parse this link in relation to that.
//
// For example if `u` is just a path, and context is a normal URI, we'll copy
// the host and scheme over from context.
func ParseLink(u string, context *url.URL) *url.URL {
	parsedUri, err := url.Parse(u)
	if err != nil {
		log.Printf("%+v", err)
	}

	if context != nil {
		// If URI is just a fragment or a path, fix links based off of the paged
		// they were on. Won't work for relative links I think...
		if parsedUri.Host == "" {
			parsedUri.Host = context.Host
			parsedUri.Scheme = context.Scheme
		}
	}

	if parsedUri.Scheme == "" {
		parsedUri.Scheme = "http"
	}

	if parsedUri.Path == "" {
		parsedUri.Path = "/"
	}

	return parsedUri
}

func main() {

	if len(os.Args) < 2 {
		fmt.Println(`
hayden: Link archiver.

  usage: hayden http://example.com

Pass in one link, and hayden will scrape it and submit it and everything the
page links to to Internet Archive.
`)
		os.Exit(1)
	}

	link := ParseLink(os.Args[1], nil)

	urls := GetLinks(link)
	for _, v := range urls {
		fmt.Println(v)
	}
}
