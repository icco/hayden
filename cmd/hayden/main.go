// A tool for archiving links in a page.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/icco/hayden"
)

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

	link := hayden.ParseLink(os.Args[1], nil)

	urls := hayden.GetLinks(link)
	for _, v := range urls {
		log.Println(v)
		links, err := hayden.SaveLink(v)
		if err != nil {
			log.Fatalf("%+v", err)
		}
		log.Printf("%+v", links)
	}
}
