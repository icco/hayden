package main

import (
	_ "embed"

	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/icco/gutil/logging"
	"github.com/icco/hayden"
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

	//go:embed config.json
	configFile []byte
)

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	cf, err := hayden.ParseConfigFile(configFile)
	if err != nil {
		log.Fatalw("could not parse config file", "configfile", configFile, zap.Error(err))
	}
	cf.Config.Log = log

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(logging.Middleware(log.Desugar(), "icco-cloud"))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok.")); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("root").Parse(rootTmpl)
		if err != nil {
			log.Errorw("could not parse template", zap.Error(err))
		}

		if err := tmpl.Execute(w, nil); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}

		if _, err := w.Write([]byte("ok.")); err != nil {
			log.Errorw("could not write response", zap.Error(err))
		}
	})

	r.Get("/force", func(w http.ResponseWriter, r *http.Request) {
		if err := cf.ScrapeTargets(r.Context()); err != nil {
			log.Errorw("could not scrape", zap.Error(err))
		}
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
