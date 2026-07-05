package hayden

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// Config holds service-wide defaults shared across targets.
type Config struct {
	Log           *zap.SugaredLogger `json:"-"`
	DefaultHook   string             `json:"default-hook"`
	DefaultPeriod int                `json:"default-period"`
}

// ConfigFile is the on-disk representation of the service config and its
// targets.
type ConfigFile struct {
	Config  *Config   `json:"config"`
	Targets []*Target `json:"targets"`
}

// ParseConfigFile decodes a ConfigFile from JSON bytes.
func ParseConfigFile(stream []byte) (*ConfigFile, error) {
	var cf ConfigFile
	if err := json.Unmarshal(stream, &cf); err != nil {
		return nil, err
	}
	if cf.Config == nil {
		cf.Config = &Config{}
	}

	return &cf, nil
}

// SeedConfig inserts the config file's targets into an empty store, applying
// defaults. It is a no-op when the store already has targets, so it only takes
// effect on a fresh database. Legacy targets default to headless fetching to
// preserve prior behavior.
func SeedConfig(ctx context.Context, store *Store, cf *ConfigFile) (int, error) {
	n, err := store.Count(ctx)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, nil
	}

	seeded := 0
	for _, t := range cf.Targets {
		if t.MatchType == "" {
			t.MatchType = "substring"
		}
		if t.FetchMode == "" {
			t.FetchMode = "headless"
		}
		if t.NotifyMode == "" {
			t.NotifyMode = "once"
		}
		t.Enabled = true
		if err := store.Create(ctx, t); err != nil {
			return seeded, fmt.Errorf("seeding target %q: %w", t.Name, err)
		}
		seeded++
	}

	return seeded, nil
}
