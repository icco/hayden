// Command server runs the hayden HTTP service: watch targets in Postgres,
// scanned on a schedule, firing a webhook on a match, managed via a small API.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/icco/gutil/logging"
	"github.com/icco/hayden"
	"github.com/icco/hayden/server/static"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

var (
	log = logging.Must(logging.NewLogger(hayden.Service))

	rootTemplate = template.Must(template.New("root").Parse(`
<html>
<head>
<title>Hayden</title>
</head>
<body>
<h1>Scraper!</h1>
<p>Watching web pages and firing webhooks on a match.</p>
</body>
</html>
`))
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	databaseURL := fs.String("database_url", "", "Postgres connection string (env: DATABASE_URL).")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalw("error parsing flags", zap.Error(err))
	}
	if *databaseURL == "" {
		log.Fatalw("database_url is required (set DATABASE_URL)")
	}

	apiToken := os.Getenv("HAYDEN_API_TOKEN")
	if apiToken == "" {
		log.Warnw("HAYDEN_API_TOKEN is unset; /targets writes and /force are unauthenticated")
	}

	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		log.Fatalw("could not create metrics exporter", zap.Error(err))
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(mp)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		//nolint:contextcheck // fresh timeout context for shutdown
		if err := mp.Shutdown(shutdownCtx); err != nil {
			log.Warnw("meter provider shutdown", zap.Error(err))
		}
	}()

	gdb, err := hayden.Connect(ctx, *databaseURL)
	if err != nil {
		log.Fatalw("could not connect to database", zap.Error(err))
	}
	if err := hayden.AutoMigrate(ctx, gdb); err != nil {
		log.Fatalw("could not migrate database", zap.Error(err))
	}
	store := hayden.NewStore(gdb)

	cf := loadConfig()
	cf.Config.Log = log
	if seeded, err := hayden.SeedConfig(ctx, store, cf); err != nil {
		log.Fatalw("could not seed config", zap.Error(err))
	} else if seeded > 0 {
		log.Infow("seeded targets from config", "count", seeded)
	}

	notifier := hayden.HTTPNotifier{
		Client:      &http.Client{Timeout: 15 * time.Second},
		DefaultHook: cf.Config.DefaultHook,
	}
	scanner := &hayden.Scanner{Store: store, Notifier: notifier}
	scheduler := &hayden.Scheduler{Scanner: scanner, Store: store, Cfg: cf.Config, Log: log}
	if err := scheduler.Start(ctx); err != nil {
		log.Fatalw("could not start scheduler", zap.Error(err))
	}
	defer scheduler.Stop()

	handler := otelhttp.NewHandler(
		router(ctx, store, scanner, scheduler, apiToken, registry),
		"hayden",
		otelhttp.WithFilter(func(req *http.Request) bool { return req.URL.Path != "/metrics" }),
	)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		//nolint:contextcheck // fresh timeout context; the signal ctx is already canceled
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warnw("server shutdown", zap.Error(err))
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalw("server error", zap.Error(err))
	}
	log.Infow("shut down cleanly")
}

func loadConfig() *hayden.ConfigFile {
	b, err := static.Content.ReadFile("config.json")
	if err != nil {
		log.Fatalw("could not read config file", zap.Error(err))
	}
	cf, err := hayden.ParseConfigFile(b)
	if err != nil {
		log.Fatalw("could not parse config file", zap.Error(err))
	}
	return cf
}

func router(baseCtx context.Context, store *hayden.Store, scanner *hayden.Scanner, scheduler *hayden.Scheduler, apiToken string, registry *prometheus.Registry) http.Handler {
	r := chi.NewRouter()
	r.Use(logging.Middleware(log.Desugar()))

	r.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeText(w, "ok.")
	})

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		if err := rootTemplate.Execute(w, nil); err != nil {
			log.Errorw("could not render root", zap.Error(err))
		}
	})

	r.Get("/targets", func(w http.ResponseWriter, r *http.Request) {
		targets, err := store.List(r.Context())
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, targets)
	})

	r.Handle("/favicon.ico", http.FileServer(http.FS(static.Content)))

	// Mutating routes cause outbound fetches; gate them behind the API token.
	r.Group(func(pr chi.Router) {
		if apiToken != "" {
			pr.Use(requireToken(apiToken))
		}

		pr.Post("/force", func(w http.ResponseWriter, _ *http.Request) {
			go func() { //nolint:contextcheck // scan outlives the request, bounded by baseCtx
				if err := scanner.ScanAll(baseCtx); err != nil {
					log.Warnw("force scan", zap.Error(err))
				}
			}()
			writeText(w, "ok.")
		})

		pr.Post("/targets", func(w http.ResponseWriter, r *http.Request) {
			var t hayden.Target
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			if t.URL == "" {
				http.Error(w, "url is required", http.StatusBadRequest)
				return
			}
			if t.MatchType == "" {
				t.MatchType = "substring"
			}
			matcher, err := hayden.MatcherFor(t.MatchType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := matcher.Validate(t.MatchValue); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if t.FetchMode == "" {
				t.FetchMode = "http"
			}
			if _, err := hayden.FetcherFor(t.FetchMode); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if t.NotifyMode == "" {
				t.NotifyMode = "once"
			}
			// Server owns identity and run-state; ignore anything the client sent.
			t.ID = 0
			t.Enabled = true
			t.LastRunAt, t.LastMatchAt = nil, nil
			t.LastStatus, t.LastError, t.LastContentHash = "", "", ""
			t.LastMatched = false

			if err := store.Create(r.Context(), &t); err != nil {
				httpError(w, err, http.StatusInternalServerError)
				return
			}
			if err := scheduler.Reload(r.Context()); err != nil {
				log.Warnw("scheduler reload after create", zap.Error(err))
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, &t)
		})

		pr.Delete("/targets/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
			if err != nil {
				http.Error(w, "invalid id", http.StatusBadRequest)
				return
			}
			if err := store.Delete(r.Context(), uint(id)); err != nil {
				httpError(w, err, http.StatusInternalServerError)
				return
			}
			if err := scheduler.Reload(r.Context()); err != nil {
				log.Warnw("scheduler reload after delete", zap.Error(err))
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	return r
}

// requireToken rejects requests without a matching Bearer token.
func requireToken(token string) func(http.Handler) http.Handler {
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeText(w http.ResponseWriter, s string) {
	if _, err := w.Write([]byte(s)); err != nil {
		log.Errorw("could not write response", zap.Error(err))
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Errorw("could not encode json", zap.Error(err))
	}
}

func httpError(w http.ResponseWriter, err error, code int) {
	log.Errorw("request error", zap.Error(err))
	http.Error(w, http.StatusText(code), code)
}
