package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultSystemPrompt = "Ты — проверяющий тестовых заданий. На вход подаётся вопрос и варианты ответа. Отвечай ТОЛЬКО номером(ами) правильного варианта(ов). Если правильных вариантов несколько, перечисли их подряд или через слэш/запятую, например: 2 или 1/3. Не выводи никакой другой текст, пояснения, слова, разметку или пунктуацию — только цифры."

type Config struct {
	APIKey            string  `json:"api_key"`
	BaseURL           string  `json:"base_url"`
	Model             string  `json:"model"`
	SystemPrompt      string  `json:"system_prompt"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	Thinking          bool    `json:"thinking"`
	MinTextLength     int     `json:"min_text_length"`
	PollIntervalMs    int     `json:"poll_interval_ms"`
	RequestTimeoutSec int     `json:"request_timeout_sec"`
	WebSearch         bool    `json:"web_search"`
	StateFile         string  `json:"state_file"`
	HistoryFile       string  `json:"history_file"`
	ImageDir          string  `json:"image_dir"`
	InboxDir          string  `json:"inbox_dir"`
	LogFile           string  `json:"log_file"`
}

func defaultConfig() Config {
	return Config{
		BaseURL:           "https://api.deepseek.com",
		Model:             "deepseek-flash",
		SystemPrompt:      defaultSystemPrompt,
		Temperature:       0,
		MaxTokens:         16,
		MinTextLength:     4,
		PollIntervalMs:    350,
		RequestTimeoutSec: 30,
		StateFile:         "~/.cache/answerd/latest.json",
		HistoryFile:       "~/.cache/answerd/history.jsonl",
		ImageDir:          "~/.cache/answerd/images",
		InboxDir:          "~/.cache/answerd/inbox",
		LogFile:           "~/.cache/answerd/answerd.log",
	}
}

func LoadConfig(path string) (Config, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return cfg, err
	}
	if cfg.APIKey == "" {
		return cfg, errors.New("DeepSeek API key is not set: set DEEPSEEK_API_KEY or api_key in config.json")
	}
	return cfg, nil
}

// LoadConfigPaths loads the config without requiring an API key. It is used by
// the lightweight screenshot subcommand.
func LoadConfigPaths(path string) (Config, error) {
	return loadConfig(path)
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	if path == "" {
		path = os.Getenv("ANSWERD_CONFIG")
	}
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, ".config", "answerd", "config.json")
		}
	}

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			// Overlay present keys onto the defaults (absent fields are kept).
			if uerr := json.Unmarshal(data, &cfg); uerr != nil {
				return cfg, fmt.Errorf("invalid config %s: %w", path, uerr)
			}
		case !os.IsNotExist(err):
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		}
	}

	// Environment variable takes precedence over the config file.
	if env := os.Getenv("DEEPSEEK_API_KEY"); env != "" {
		cfg.APIKey = env
	}

	cfg.StateFile = expandHome(cfg.StateFile)
	cfg.HistoryFile = expandHome(cfg.HistoryFile)
	cfg.ImageDir = expandHome(cfg.ImageDir)
	cfg.InboxDir = expandHome(cfg.InboxDir)
	cfg.LogFile = expandHome(cfg.LogFile)
	return cfg, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
