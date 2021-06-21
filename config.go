package hayden

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"
)

type Config struct {
	Log           *zap.SugaredLogger `json:"-"`
	DefaultHook   string             `json:"default-hook"`
	DefaultPeriod int                `json:"default-period"`
}

type ConfigFile struct {
	Config  *Config   `json:"config"`
	Targets []*Target `json:"targets"`
}

func ParseConfigFile(stream []byte) (*ConfigFile, error) {
	var cf ConfigFile
	if err := json.Unmarshal(stream, &cf); err != nil {
		return nil, err
	}

	return &cf, nil
}

func (cf *ConfigFile) ScrapeTargets(ctx context.Context) error {
	for _, t := range cf.Targets {
		match, err := t.Scan(ctx, cf.Config)
		if err != nil {
			cf.Config.Log.Errorw("error on target", "target", t, zap.Error(err))
			continue
		}

		if match {
			cf.Config.Log.Infow("success on scan", "target", t)
		}
	}

	return nil
}
