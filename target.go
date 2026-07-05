package hayden

import (
	"time"

	"gorm.io/gorm"
)

// Target is a single web page to watch, the match that triggers its hook, and
// the persisted state of its most recent scan.
type Target struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name       string `gorm:"not null" json:"name"`
	URL        string `gorm:"not null" json:"url"`
	FetchMode  string `gorm:"not null;default:http" json:"fetch_mode"`      // http | headless
	MatchType  string `gorm:"not null;default:substring" json:"match_type"` // substring (css|regex|jsonpath in a later phase)
	MatchValue string `json:"match_value"`
	Invert     bool   `json:"invert"`
	NotifyMode string `gorm:"not null;default:once" json:"notify_mode"` // once (change in a later phase)
	Hook       string `json:"hook,omitempty"`
	Period     int    `json:"period,omitempty"` // seconds; 0 → Config.DefaultPeriod
	Enabled    bool   `gorm:"not null;default:true" json:"enabled"`

	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastStatus      string     `json:"last_status,omitempty"` // ok | error
	LastMatchAt     *time.Time `json:"last_match_at,omitempty"`
	LastMatched     bool       `json:"last_matched"`
	LastError       string     `json:"last_error,omitempty"`
	LastContentHash string     `json:"-"`
}

// EffectiveHook returns the target's own hook, falling back to the service
// default.
func (t *Target) EffectiveHook(cfg *Config) string {
	if t.Hook != "" {
		return t.Hook
	}
	return cfg.DefaultHook
}

// EffectivePeriod returns the scan interval, falling back to the service
// default and finally to five minutes.
func (t *Target) EffectivePeriod(cfg *Config) time.Duration {
	secs := t.Period
	if secs <= 0 {
		secs = cfg.DefaultPeriod
	}
	if secs <= 0 {
		secs = 300
	}
	return time.Duration(secs) * time.Second
}
