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

// Delete soft-deletes a target by id (gorm.ErrRecordNotFound if missing).
func (s *Store) Delete(ctx context.Context, id uint) error {
	res := s.DB.WithContext(ctx).Delete(&Target{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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

// targetConfigColumns are the client-editable columns of a target.
var targetConfigColumns = []string{"name", "url", "fetch_mode", "match_type", "match_value", "invert", "notify_mode", "hook", "period", "enabled"}

// Update persists the client-editable columns of t, including zero values.
func (s *Store) Update(ctx context.Context, t *Target) error {
	return s.DB.WithContext(ctx).Model(t).Select(targetConfigColumns).Updates(t).Error
}

// historyLimit is how many scan records each target keeps.
const historyLimit = 100

// RecordScan appends r to its target's history and prunes entries past historyLimit.
func (s *Store) RecordScan(ctx context.Context, r *ScanRecord) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(r).Error; err != nil {
			return err
		}
		keep := tx.Model(&ScanRecord{}).Select("id").Where("target_id = ?", r.TargetID).Order("id desc").Limit(historyLimit)
		return tx.Where("target_id = ? AND id NOT IN (?)", r.TargetID, keep).Delete(&ScanRecord{}).Error
	})
}

// ListScans returns up to limit of a target's most recent scans, newest first.
func (s *Store) ListScans(ctx context.Context, targetID uint, limit int) ([]*ScanRecord, error) {
	var rs []*ScanRecord
	err := s.DB.WithContext(ctx).Where("target_id = ?", targetID).Order("id desc").Limit(limit).Find(&rs).Error
	return rs, err
}
