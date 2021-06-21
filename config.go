package hayden

import "go.uber.org/zap"

type Config struct {
	Log           *zap.SugaredLogger `json:"-"`
	DefaultHook   string             `json:"default-hook"`
	DefaultPeriod int                `json:"default-period"`
}

type ConfigFile struct {
	Config  *Config   `json:"config"`
	Targets []*Target `json:"targets"`
}
