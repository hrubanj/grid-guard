package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsAndParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := `{
	  "solax": {
	    "loginData": {
	      "loginPayload": {"username": "u@example.com", "userpwd": "abc123hash"},
	      "deviceLoginPayload": {"sn": "SN1", "userType": "9", "userId": "42", "firmId": "7"}
	    },
	    "paramsBasePayload": {"optType": "setReg", "num": 86, "sn": "SN1", "inverterSn": "SN1", "deviceType": 99},
	    "monitoringToken": "montok"
	  },
	  "telegram": {"bot_token": "bt", "chat_id": "-100"},
	  "threshold_eur_mwh": 0,
	  "lookahead_minutes": 5,
	  "timezone": "Europe/Prague",
	  "allow_export_value": 13000
	}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Telegram.BotToken != "bt" || cfg.Telegram.ChatID != "-100" {
		t.Errorf("telegram parsed wrong: %+v", cfg.Telegram)
	}
	if cfg.LookaheadMinutes != 5 {
		t.Errorf("lookahead = %d, want 5", cfg.LookaheadMinutes)
	}
	if cfg.AllowExportValue != 13000 {
		t.Errorf("allow_export_value = %d, want 13000", cfg.AllowExportValue)
	}
	if cfg.Solax.LoginData.LoginPayload["userpwd"] != "abc123hash" {
		t.Errorf("login payload parsed wrong: %+v", cfg.Solax.LoginData.LoginPayload)
	}
	if got := cfg.Solax.ParamsBasePayload["num"]; got == nil {
		t.Errorf("paramsBasePayload missing num")
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Minimal config omitting lookahead_minutes, timezone, and allow_export_value
	// so that LoadConfig must fill in the defaults.
	raw := `{"telegram":{"bot_token":"x","chat_id":"y"}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.LookaheadMinutes != 5 {
		t.Errorf("LookaheadMinutes = %d, want 5", cfg.LookaheadMinutes)
	}
	if cfg.Timezone != "Europe/Prague" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "Europe/Prague")
	}
	if cfg.AllowExportValue != 15000 {
		t.Errorf("AllowExportValue = %d, want 15000", cfg.AllowExportValue)
	}
	if cfg.Language != "en" {
		t.Errorf("Language = %q, want %q (default)", cfg.Language, "en")
	}
}

func TestLoadConfigRejectsUnknownLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := `{"telegram":{"bot_token":"x","chat_id":"y"},"language":"de"}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig accepted unsupported language, want error")
	}
}
