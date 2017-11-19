// A simple webserver for archiving links in a page.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/icco/hayden"
)

func postHandler(w http.ResponseWriter, r *http.Request) {
	link := hayden.ParseLink(r.PostFormValue("link"), nil)
	urls := hayden.GetLinks(link)
	for _, v := range urls {
		fmt.Println(v)
		hayden.SaveLink(v)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<!doctype html>
<html>
  <head>
    <link rel="stylesheet" href="https://unpkg.com/tachyons@4.9.0/css/tachyons.min.css"/>
    <title>Hayden Archive Tool</title>
  </head>
  <body>
    <h1>Hayden Archive Tool</h1>
    <div class="pa4-l">
      <form method="post" action="/api/save" class="bg-light-red mw7 center pa4 br2-ns ba b--black-10">
        <fieldset class="cf bn ma0 pa0">
          <legend class="pa0 f5 f4-ns mb3 black-80">Archive links on this page</legend>
          <div class="cf">
            <label class="clip" for="archive-url">URL to archive</label>
            <input class="f6 f5-l input-reset bn fl black-80 bg-white pa3 lh-solid w-100 w-75-m w-80-l br2-ns br--left-ns" placeholder="URL to archive" type="text" name="archive-url" value="" id="archive-url">
            <input class="f6 f5-l button-reset fl pv3 tc bn bg-animate bg-black-70 hover-bg-black white pointer w-100 w-25-m w-20-l br2-ns br--right-ns" type="submit" value="Subscribe">
          </div>
        </fieldset>
      </form>
    </div>
  </body>
</html>
`)
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/", postHandler)
	log.Printf("Starting server on localhost:8080")
	http.ListenAndServe(":8080", nil)
}
