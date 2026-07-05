// Command migrate connects to Postgres and syncs the schema. The server also
// auto-migrates on startup; this exists for manual or CI migration runs.
package main

import (
	"context"
	"log"
	"os"

	"github.com/icco/hayden"
	"github.com/namsral/flag"
)

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "HAYDEN", 0)
	databaseURL := fs.String("database_url", "", "Postgres connection string.")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("parsing flags: %v", err)
	}
	if *databaseURL == "" {
		log.Fatal("database_url is required (set HAYDEN_DATABASE_URL)")
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
