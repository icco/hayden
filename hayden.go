// A set of functions for archiving links in a page.
package hayden

import (
	"log"
	"net/http"
	"net/url"

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

// This takes a single link and submits it to the Internet Archive for storage.
//
// NOTE: We assume the passed in link has already been made a nice and properly
// formatted HTTP or HTTPS url. If it has not, this will fail.
func SaveLink(toSave string) error {
	iaUrl := fmt.Sprintf("https://web.archive.org/save/%s", toSave)

	rs, err := http.Get(iaUrl)
	if err != nil {
		log.Printf("Error while archiving: %+v", err)
		return err
	}
	defer rs.Body.Close()

	log.Printf("Response Status (%s): %+v", iaUrl, rs.Status)
	log.Printf("Response Headers (%s): %+v", iaUrl, rs.Header)

	return nil
}
