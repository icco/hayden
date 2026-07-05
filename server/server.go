// Command server runs the hayden HTTP service: it serves health checks and a
// status page, and triggers target scrapes on demand.
package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/icco/gutil/logging"
	"github.com/icco/hayden"
	"github.com/icco/hayden/server/static"
	"go.uber.org/zap"
)

var (
	log = logging.Must(logging.NewLogger(hayden.Service))

	rootTmpl = `
<html>
<head>
<title>Hayden</title>
</head>
<body>
<h1>Scraper!</h1>
</body>
</html>
`
)

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	configFile, err := static.Content.ReadFile("config.json")
	if err != nil {
		log.Fatalw("could not read config file", zap.Error(err))
	}
	cf, err := hayden.ParseConfigFile(configFile)
	if err != nil {
		log.Fatalw("could not parse config file", "configfile", configFile, zap.Error(err))
	}
	cf.Config.Log = log
	log.Debugw("loaded config", "config", cf)

	r := chi.NewRouter()
	r.Use(logging.Middleware(log.Desugar()))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("ok.")); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}
	})

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		tmpl, err := template.New("root").Parse(rootTmpl)
		if err != nil {
			log.Errorw("could not parse template", zap.Error(err))
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}
	})

	r.Handle("/favicon.ico", http.FileServer(http.FS(static.Content)))

	r.Get("/force", func(w http.ResponseWriter, _ *http.Request) {
		// Scanning is rewired onto the store-backed scanner alongside the
		// scheduler; this handler is a placeholder until then.
		if _, err := w.Write([]byte("ok.")); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
