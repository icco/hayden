// A set of functions for archiving links in a page.
package hayden

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

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

func SaveLink(toSave string) (string, error) {

}

// This takes a single link and submits it to Archive.is for storage.
//
// NOTE: We assume the passed in link has already been made a nice and properly
// formatted HTTP or HTTPS url. If it has not, this will fail.
func SaveToArchiveIs(toSave string) (string, error) {
	aUrl := fmt.Sprintf("https://archive.today/?run=1&url=%s", toSave)

	rs, err := http.Get(aUrl)
	if err != nil {
		log.Printf("Error while archiving %s: %+v", aUrl, err)
		return "", err
	}
	defer rs.Body.Close()

	parsedAUrl, err := url.Parse(aUrl)
	if err != nil {
		log.Printf("Error while parsing %s: %+v", aUrl, err)
		return "", err
	}
	savedLocation := ParseLink(strings.Join(rs.Header["Content-Location"], ""), parsedAUrl)

	log.Printf("Response Status (%s): %+v", aUrl, rs.Status)

	return savedLocation.String(), nil
}

// This takes a single link and submits it to the Internet Archive for storage.
//
// NOTE: We assume the passed in link has already been made a nice and properly
// formatted HTTP or HTTPS url. If it has not, this will fail.
func SaveToInternetArchvive(toSave string) (string, error) {
	iaUrl := fmt.Sprintf("https://web.archive.org/save/%s", toSave)

	// Create custom client because IA returns 30x if there has been a recent
	// snapshot, and it is a redirect loop.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	rs, err := client.Get(iaUrl)
	if err != nil {
		log.Printf("Error while archiving %s: %+v", iaUrl, err)
		return "", err
	}
	defer rs.Body.Close()

	parsedIaUrl, err := url.Parse(iaUrl)
	if err != nil {
		log.Printf("Error while parsing %s: %+v", iaUrl, err)
		return "", err
	}
	savedLocation := ParseLink(strings.Join(rs.Header["Content-Location"], ""), parsedIaUrl)

	log.Printf("Response Status (%s): %+v", iaUrl, rs.Status)

	return savedLocation.String(), nil
}
