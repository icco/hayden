package hayden

import (
	"context"
	"os"
	"testing"

	"gorm.io/gorm"
)

// testDB connects to TEST_DATABASE_URL and migrates the schema, skipping the
// test when the env var is unset. It drops the targets table on cleanup.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS targets") })

	return db
}

func TestConnectAndMigrate(t *testing.T) {
	_ = testDB(t)
}
