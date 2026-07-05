package hayden

import (
	"context"

	"gorm.io/gorm"
)

// Store persists targets and their run-state in Postgres.
type Store struct{ DB *gorm.DB }

// NewStore wraps a gorm DB.
func NewStore(db *gorm.DB) *Store { return &Store{DB: db} }

// Create inserts a new target.
func (s *Store) Create(ctx context.Context, t *Target) error {
	return s.DB.WithContext(ctx).Create(t).Error
}

// List returns all targets in id order.
func (s *Store) List(ctx context.Context) ([]*Target, error) {
	var ts []*Target
	err := s.DB.WithContext(ctx).Order("id asc").Find(&ts).Error
	return ts, err
}

// ListEnabled returns only enabled targets, in id order.
func (s *Store) ListEnabled(ctx context.Context) ([]*Target, error) {
	var ts []*Target
	err := s.DB.WithContext(ctx).Where("enabled = ?", true).Order("id asc").Find(&ts).Error
	return ts, err
}

// Get returns a target by id (gorm.ErrRecordNotFound if missing).
func (s *Store) Get(ctx context.Context, id uint) (*Target, error) {
	var t Target
	if err := s.DB.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// Delete soft-deletes a target by id.
func (s *Store) Delete(ctx context.Context, id uint) error {
	return s.DB.WithContext(ctx).Delete(&Target{}, id).Error
}

// SaveRunState persists only the run-state columns of t, including zero values.
func (s *Store) SaveRunState(ctx context.Context, t *Target) error {
	return s.DB.WithContext(ctx).Model(t).Updates(map[string]any{
		"last_run_at":       t.LastRunAt,
		"last_status":       t.LastStatus,
		"last_match_at":     t.LastMatchAt,
		"last_matched":      t.LastMatched,
		"last_error":        t.LastError,
		"last_content_hash": t.LastContentHash,
	}).Error
}

// Count returns the number of non-deleted targets.
func (s *Store) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.DB.WithContext(ctx).Model(&Target{}).Count(&n).Error
	return n, err
}
