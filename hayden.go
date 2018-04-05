// Package hayden provides a set of functions for archiving links in a page
// and uploading them to various archiving services.
package hayden

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/fatih/set.v0"
)

// GetLinks takes a url, scrapes and grabs all things linked with an anchor
// tag. It returns those in a list of unique strings.
func GetLinks(baseURI *url.URL) []string {
	s := set.New()
	s.Add(baseURI.String())

	doc, err := goquery.NewDocument(baseURI.String())
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body a").Each(func(index int, item *goquery.Selection) {
		linkTag := item
		link, _ := linkTag.Attr("href")
		log.Printf("%d: %s", index, link)

		// Add to final set.
		s.Add(ParseLink(link, baseURI).String())
	})

	return set.StringSlice(s)
}

// ParseLink takes a uri string, return a nicely filled out URL. If context is
// provided, we'll parse this link in relation to that.
//
// For example if `u` is just a path, and context is a normal URI, we'll copy
// the host and scheme over from context.
func ParseLink(u string, context *url.URL) *url.URL {
	parsedURI, err := url.Parse(u)
	if err != nil {
		log.Printf("%+v", err)
	}

	if context != nil {
		// If URI is just a fragment or a path, fix links based off of the paged
		// they were on. Won't work for relative links I think...
		if parsedURI.Host == "" {
			parsedURI.Host = context.Host
			parsedURI.Scheme = context.Scheme
		}
	}

	if parsedURI.Scheme == "" {
		parsedURI.Scheme = "http"
	}

	if parsedURI.Path == "" {
		parsedURI.Path = "/"
	}

	return parsedURI
}

// RandomString takes a number and returns a string with random a-Z characters
// of that length.
func RandomString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

// SaveLink is a wrapper around SaveToArchiveIs and SaveToInternetArchive.
func SaveLink(toSave string) ([]string, error) {

	iaURL, err := SaveToInternetArchive(toSave)
	if err != nil {
		return nil, err
	}

	aURL, err := SaveToArchiveIs(toSave)
	if err != nil {
		return nil, err
	}

	return []string{iaURL, aURL}, nil
}

// SaveToArchiveIs takes a single link and submits it to Archive.is for
// storage.
//
// NOTE: We assume the passed in link has already been made a nice and properly
// formatted HTTP or HTTPS url. If it has not, this will fail.
//
// NOTE: This functionality is currently broken, it will not save a new
// snapshot if one has ever been made.
func SaveToArchiveIs(toSave string) (string, error) {
	now := time.Now()
	aURL := fmt.Sprintf("https://archive.is/submit/")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	rs, err := client.PostForm(aURL, url.Values{
		"url":      {toSave},
		"submitid": {RandomString(65)},
	})
	if err != nil {
		log.Printf("Error while archiving %s: %+v", aURL, err)
		return "", err
	}
	defer rs.Body.Close()

	log.Printf("Response Status (%s): %+v", aURL, rs.Status)

	archiveURL := fmt.Sprintf("https://archive.is/%s/%s", now.Format("200601021504"), toSave)

	return archiveURL, nil
}

// SaveToInternetArchive takes a single link and submits it to the Internet
// Archive for storage.
//
// NOTE: We assume the passed in link has already been made a nice and properly
// formatted HTTP or HTTPS url. If it has not, this will fail.
func SaveToInternetArchive(toSave string) (string, error) {
	iaURL := fmt.Sprintf("https://web.archive.org/save/%s", toSave)

	// Create custom client because IA returns 30x if there has been a recent
	// snapshot, and it is a redirect loop.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	rs, err := client.Get(iaURL)
	if err != nil {
		log.Printf("Error while archiving %s: %+v", iaURL, err)
		return "", err
	}
	defer rs.Body.Close()

	parsedIaURL, err := url.Parse(iaURL)
	if err != nil {
		log.Printf("Error while parsing %s: %+v", iaURL, err)
		return "", err
	}
	savedLocation := ParseLink(strings.Join(rs.Header["Content-Location"], ""), parsedIaURL)

	log.Printf("Response Status (%s): %+v", iaURL, rs.Status)

	return savedLocation.String(), nil
}
