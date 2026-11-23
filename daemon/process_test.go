package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig(dir string) Config {
	cfg := defaultConfig()
	cfg.APIKey = "test"
	cfg.BaseURL = "http://127.0.0.1:1" // connection refused -> fast error, no real API
	cfg.RequestTimeoutSec = 1
	cfg.StateFile = filepath.Join(dir, "latest.json")
	cfg.HistoryFile = filepath.Join(dir, "history.jsonl")
	cfg.ImageDir = filepath.Join(dir, "images")
	return cfg
}

func TestProcessTextErrorRecorded(t *testing.T) {
	cfg := testConfig(t.TempDir())
	state, _ := NewState(cfg.StateFile)
	history, _ := NewHistory(cfg.HistoryFile, cfg.ImageDir)
	client := NewDeepSeekClient(cfg)

	item := &ClipboardItem{Kind: "text", Data: []byte("вопрос 1) a 2) b"), Hash: "abc"}
	process(context.Background(), client, state, history, item)

	data, err := os.ReadFile(cfg.HistoryFile)
	if err != nil {
		t.Fatalf("history not written: %v", err)
	}
	if !strings.Contains(string(data), "\"status\":\"error\"") {
		t.Fatalf("expected error record, got: %s", data)
	}
	if !strings.Contains(string(data), "вопрос") {
		t.Fatalf("expected question in history, got: %s", data)
	}
}

func TestProcessImageSavedAndRecorded(t *testing.T) {
	cfg := testConfig(t.TempDir())
	state, _ := NewState(cfg.StateFile)
	history, _ := NewHistory(cfg.HistoryFile, cfg.ImageDir)
	client := NewDeepSeekClient(cfg)

	item := &ClipboardItem{Kind: "image", Data: []byte("PNGDATA"), Hash: "def"}
	process(context.Background(), client, state, history, item)

	entries, err := os.ReadDir(cfg.ImageDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("image not saved: %v", err)
	}

	data, err := os.ReadFile(cfg.HistoryFile)
	if err != nil {
		t.Fatalf("history not written: %v", err)
	}
	if !strings.Contains(string(data), "image_path") {
		t.Fatalf("expected image_path in history, got: %s", data)
	}
}
