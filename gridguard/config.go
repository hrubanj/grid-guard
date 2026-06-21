package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// SolaxSecrets mirrors the solax_secrets block of the legacy app_config.json.
type SolaxSecrets struct {
	LoginData struct {
		LoginPayload       map[string]string `json:"loginPayload"`
		DeviceLoginPayload map[string]string `json:"deviceLoginPayload"`
	} `json:"loginData"`
	ParamsBasePayload map[string]any `json:"paramsBasePayload"`
	MonitoringToken   string         `json:"monitoringToken"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type Config struct {
	Solax    SolaxSecrets   `json:"solax"`
	Telegram TelegramConfig `json:"telegram"`
	// ThresholdEurMWh: forbid selling when price <= this; 0 (the default) means forbid at non-positive prices.
	ThresholdEurMWh  float64 `json:"threshold_eur_mwh"`
	LookaheadMinutes int     `json:"lookahead_minutes"`
	Timezone         string  `json:"timezone"`
	// AlsoSetWorkmode: when true, also set WORK_MODE (SELF_USE when forbidding, FEED_IN when allowing) in addition to the export cap.
	AlsoSetWorkmode  bool `json:"also_set_workmode"`
	AllowExportValue int  `json:"allow_export_value"`
	// Language selects the Telegram notification language: "en" (default) or "cs". See messages.go.
	Language string `json:"language"`
}

// LoadConfig reads the JSON config and applies sane defaults.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.LookaheadMinutes == 0 {
		cfg.LookaheadMinutes = 5
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Europe/Prague"
	}
	if cfg.AllowExportValue == 0 {
		cfg.AllowExportValue = 15000
	}
	if cfg.AllowExportValue < 10 {
		return nil, fmt.Errorf("allow_export_value must be >= 10 W (export cap is encoded in tens of watts)")
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	if _, ok := bundles[cfg.Language]; !ok {
		return nil, fmt.Errorf("language %q not supported (have %v)", cfg.Language, supportedLanguages())
	}
	return &cfg, nil
}
