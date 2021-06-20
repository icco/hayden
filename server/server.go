package main

import (
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
)

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	cfg := &hayden.Config{Log: log}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(logging.Middleware(log.Desugar(), "icco-cloud"))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("ok."))
		if err != nil {
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
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}
