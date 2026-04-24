package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TelegramConfig struct {
	BotToken       string
	APIBase        string
	PollTimeout    time.Duration
	RequestTimeout time.Duration
	AllowedUpdates []string
}

type telegramConfigFile struct {
	BotToken       string    `json:"botToken"`
	APIBase        string    `json:"apiBase,omitempty"`
	PollTimeout    any       `json:"pollTimeout,omitempty"`
	RequestTimeout any       `json:"requestTimeout,omitempty"`
	AllowedUpdates *[]string `json:"allowedUpdates,omitempty"`
}

func defaultTelegramConfig() TelegramConfig {
	return TelegramConfig{
		APIBase:        "https://api.telegram.org",
		PollTimeout:    30 * time.Second,
		RequestTimeout: 60 * time.Second,
		AllowedUpdates: []string{"message", "edited_message", "channel_post", "edited_channel_post"},
	}
}

func parseTelegramConfigData(data []byte) (TelegramConfig, error) {
	cfg := defaultTelegramConfig()
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, nil
	}

	var raw telegramConfigFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return TelegramConfig{}, err
	}
	if strings.TrimSpace(raw.BotToken) != "" {
		cfg.BotToken = strings.TrimSpace(raw.BotToken)
	}
	if strings.TrimSpace(raw.APIBase) != "" {
		cfg.APIBase = strings.TrimRight(strings.TrimSpace(raw.APIBase), "/")
	}
	if raw.PollTimeout != nil {
		duration, err := parseDurationValue(raw.PollTimeout)
		if err != nil {
			return TelegramConfig{}, fmt.Errorf("parse pollTimeout: %w", err)
		}
		cfg.PollTimeout = duration
	}
	if raw.RequestTimeout != nil {
		duration, err := parseDurationValue(raw.RequestTimeout)
		if err != nil {
			return TelegramConfig{}, fmt.Errorf("parse requestTimeout: %w", err)
		}
		cfg.RequestTimeout = duration
	}
	if raw.AllowedUpdates != nil {
		cfg.AllowedUpdates = append([]string{}, (*raw.AllowedUpdates)...)
	}
	return cfg, nil
}

func parseDurationValue(value any) (time.Duration, error) {
	switch x := value.(type) {
	case string:
		return time.ParseDuration(strings.TrimSpace(x))
	case float64:
		return time.Duration(x), nil
	default:
		return 0, fmt.Errorf("invalid duration type %T", value)
	}
}

func (s *TelegramService) loadTelegramConfig() (TelegramConfig, error) {
	path, err := s.configPath()
	if err != nil {
		return TelegramConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return TelegramConfig{}, err
	}
	cfg, err := parseTelegramConfigData(data)
	if err != nil {
		return TelegramConfig{}, fmt.Errorf("load telegram config %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return TelegramConfig{}, fmt.Errorf("botToken is required in %s", path)
	}
	return cfg, nil
}

func (s *TelegramService) configPath() (string, error) {
	if s.app != nil {
		return s.app.Paths().ConfigPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gatecli", s.ServiceName(), "config.json"), nil
}

func writeTelegramConfigValue(path string, key string, value any) error {
	document := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("decode telegram config %s: %w", path, err)
		}
	}
	document[key] = value
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}
