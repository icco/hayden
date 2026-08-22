// Command migrate connects to Postgres and syncs the schema. The server also
// auto-migrates on startup; this exists for manual or CI migration runs.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/icco/hayden"
	"github.com/peterbourgon/ff/v3"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	databaseURL := fs.String("database_url", "", "Postgres connection string (env: DATABASE_URL).")
	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVars()); err != nil {
		log.Fatalf("parsing flags: %v", err)
	}
	if *databaseURL == "" {
		log.Fatal("database_url is required (set DATABASE_URL)")
	}

	ctx := context.Background()
	db, err := hayden.Connect(ctx, *databaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	if err := hayden.AutoMigrate(ctx, db); err != nil {
		log.Fatalf("auto-migrating: %v", err)
	}

	log.Println("migration complete")
}
