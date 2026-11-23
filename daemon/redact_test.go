package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	in := `{"api_key": "sk-abcdef1234567890", "note": "ok"}`
	got := redactSecrets(in)

	if strings.Contains(got, "abcdef1234567890") {
		t.Fatalf("secret was not redacted: %q", got)
	}
	if !strings.Contains(got, "sk-***") {
		t.Fatalf("masked key missing: %q", got)
	}
	if !strings.Contains(got, `"note": "ok"`) {
		t.Fatalf("unrelated text changed: %q", got)
	}
}

func TestLooksLikeFilePath(t *testing.T) {
	if !looksLikeFilePath("file:///home/user/Desktop/test.png") {
		t.Fatal("file URL not detected")
	}
	if looksLikeFilePath("2+2=? 1) 3 2) 4") {
		t.Fatal("ordinary question treated as a file path")
	}
}

func TestProcessRedactsSecretInHistory(t *testing.T) {
	cfg := testConfig(t.TempDir())
	state, _ := NewState(cfg.StateFile)
	history, _ := NewHistory(cfg.HistoryFile, cfg.ImageDir)
	client := NewDeepSeekClient(cfg)

	item := &ClipboardItem{
		Kind: "text",
		Data: []byte(`{"api_key": "sk-abcdef1234567890"}`),
		Hash: "secret",
	}
	process(context.Background(), client, state, history, item)

	data, err := os.ReadFile(cfg.HistoryFile)
	if err != nil {
		t.Fatalf("history not written: %v", err)
	}
	if strings.Contains(string(data), "abcdef1234567890") {
		t.Fatalf("secret leaked into history: %s", data)
	}
}
