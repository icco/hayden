package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.icco.me/hayden"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// validateTarget fills defaults and rejects configs the scanner can't run.
func validateTarget(t *hayden.Target) error {
	if err := validateURL("url", t.URL); err != nil {
		return err
	}
	if t.Hook != "" {
		if err := validateURL("hook", t.Hook); err != nil {
			return err
		}
	}
	if t.Period < 0 {
		return errors.New("period must not be negative")
	}
	if t.MatchType == "" {
		t.MatchType = "substring"
	}
	matcher, err := hayden.MatcherFor(t.MatchType)
	if err != nil {
		return err
	}
	if err := matcher.Validate(t.MatchValue); err != nil {
		return err
	}
	if t.FetchMode == "" {
		t.FetchMode = "http"
	}
	if _, err := hayden.FetcherFor(t.FetchMode); err != nil {
		return err
	}
	switch t.NotifyMode {
	case "":
		t.NotifyMode = "once"
	case "once", "change":
	default:
		return fmt.Errorf("unsupported notify mode %q", t.NotifyMode)
	}
	return nil
}

func validateURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http(s) URL", field)
	}
	return nil
}

// loadTarget resolves {id}, writing a 400 or 404 and returning nil on failure.
func loadTarget(w http.ResponseWriter, r *http.Request, store *hayden.Store) *hayden.Target {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return nil
	}
	t, err := store.Get(r.Context(), uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "target not found", http.StatusNotFound)
		return nil
	}
	if err != nil {
		httpError(w, err)
		return nil
	}
	return t
}

// targetRoutes registers the per-target routes beyond create and delete.
func targetRoutes(pr chi.Router, baseCtx context.Context, store *hayden.Store, scanner *hayden.Scanner, scheduler *hayden.Scheduler) {
	pr.Get("/targets/{id}", func(w http.ResponseWriter, r *http.Request) {
		if t := loadTarget(w, r, store); t != nil {
			writeJSON(w, t)
		}
	})

	pr.Patch("/targets/{id}", func(w http.ResponseWriter, r *http.Request) {
		orig := loadTarget(w, r, store)
		if orig == nil {
			return
		}
		// Decoding over a copy applies only the fields the client sent.
		t := *orig
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		t.ID, t.CreatedAt, t.UpdatedAt, t.DeletedAt = orig.ID, orig.CreatedAt, orig.UpdatedAt, orig.DeletedAt
		t.LastRunAt, t.LastStatus, t.LastMatchAt = orig.LastRunAt, orig.LastStatus, orig.LastMatchAt
		t.LastMatched, t.LastError, t.LastContentHash = orig.LastMatched, orig.LastError, orig.LastContentHash
		if err := validateTarget(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := store.Update(r.Context(), &t); err != nil {
			httpError(w, err)
			return
		}
		if err := scheduler.Reload(r.Context()); err != nil {
			log.Warnw("scheduler reload after update", zap.Error(err))
		}
		writeJSON(w, &t)
	})

	pr.Post("/targets/{id}/scan", func(w http.ResponseWriter, r *http.Request) {
		t := loadTarget(w, r, store)
		if t == nil {
			return
		}
		go func() { //nolint:contextcheck // scan outlives the request, bounded by baseCtx
			if err := scanner.Scan(baseCtx, t); err != nil {
				log.Warnw("manual scan", "target_id", t.ID, zap.Error(err))
			}
		}()
		w.WriteHeader(http.StatusAccepted)
		writeText(w, "ok.")
	})

	pr.Get("/targets/{id}/scans", func(w http.ResponseWriter, r *http.Request) {
		t := loadTarget(w, r, store)
		if t == nil {
			return
		}
		scans, err := store.ListScans(r.Context(), t.ID, 100)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, scans)
	})
}
