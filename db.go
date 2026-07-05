package hayden

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a Postgres connection at databaseURL and verifies it with
// PingContext.
func Connect(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting underlying connection: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	return db, nil
}

// AutoMigrate syncs the schema with the current models; safe on every startup.
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&Target{}); err != nil {
		return fmt.Errorf("auto-migrating: %w", err)
	}

	return nil
}
