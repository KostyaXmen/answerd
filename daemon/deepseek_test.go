package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskSendsThinkingDisabledAndParsesContent(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"1"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	cfg := defaultConfig()
	cfg.APIKey = "test"
	cfg.BaseURL = srv.URL

	client := NewDeepSeekClient(cfg)
	got, err := client.AskText(context.Background(), "вопрос 1) a 2) b")
	if err != nil {
		t.Fatalf("AskText: %v", err)
	}
	if got != "1" {
		t.Fatalf("content = %q, want %q", got, "1")
	}
	if !strings.Contains(string(gotBody), `"thinking":{"type":"disabled"}`) {
		t.Fatalf("thinking param missing/incorrect: %s", gotBody)
	}
}

func TestAskEmptyContentIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"","reasoning_content":"thinking..."},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	cfg := defaultConfig()
	cfg.APIKey = "test"
	cfg.BaseURL = srv.URL

	if _, err := NewDeepSeekClient(cfg).AskText(context.Background(), "q"); err == nil {
		t.Fatal("expected error on empty content")
	}
}

func captureChatRequest(t *testing.T, cfg Config) map[string]any {
	t.Helper()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("invalid request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"2"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	cfg.APIKey = "test"
	cfg.BaseURL = srv.URL
	if _, err := NewDeepSeekClient(cfg).AskText(context.Background(), "q"); err != nil {
		t.Fatalf("AskText: %v", err)
	}
	return body
}

func TestWebSearchAddsToolAndRaisesMaxTokens(t *testing.T) {
	cfg := defaultConfig()
	cfg.WebSearch = true
	cfg.MaxTokens = 16

	body := captureChatRequest(t, cfg)

	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want one deepseek_web_search tool", body["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "deepseek_web_search" {
		t.Fatalf("tool type = %v, want deepseek_web_search", tool["type"])
	}
	if got := body["max_tokens"]; got != float64(webSearchMaxTokens) {
		t.Fatalf("max_tokens = %v, want %d", got, webSearchMaxTokens)
	}
}

func TestWebSearchDisabledByDefault(t *testing.T) {
	cfg := defaultConfig()
	cfg.WebSearch = false

	body := captureChatRequest(t, cfg)

	if _, ok := body["tools"]; ok {
		t.Fatalf("tools must be omitted when web search is off, got %#v", body["tools"])
	}
	if got := body["max_tokens"]; got != float64(cfg.MaxTokens) {
		t.Fatalf("max_tokens = %v, want %d", got, cfg.MaxTokens)
	}
}

func TestThinkingEnablesAndRaisesMaxTokens(t *testing.T) {
	cfg := defaultConfig()
	cfg.Thinking = true
	cfg.MaxTokens = 16

	body := captureChatRequest(t, cfg)

	thinking, _ := body["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("thinking = %#v, want enabled", body["thinking"])
	}
	if got := body["max_tokens"]; got != float64(thinkingMaxTokens) {
		t.Fatalf("max_tokens = %v, want %d", got, thinkingMaxTokens)
	}
}

func TestThinkingWithWebSearchUsesThinkingLimit(t *testing.T) {
	cfg := defaultConfig()
	cfg.Thinking = true
	cfg.WebSearch = true
	cfg.MaxTokens = 16

	body := captureChatRequest(t, cfg)

	if _, ok := body["tools"]; !ok {
		t.Fatalf("tools missing: %#v", body)
	}
	if got := body["max_tokens"]; got != float64(thinkingMaxTokens) {
		t.Fatalf("max_tokens = %v, want %d", got, thinkingMaxTokens)
	}
}
