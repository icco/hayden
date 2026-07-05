package hayden

import (
	"encoding/json"

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

	return &cf, nil
}
