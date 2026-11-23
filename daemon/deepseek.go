package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type DeepSeekClient struct {
	cfg    Config
	client *http.Client
}

func NewDeepSeekClient(cfg Config) *DeepSeekClient {
	return &DeepSeekClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.RequestTimeoutSec) * time.Second,
		},
	}
}

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type thinkingParam struct {
	Type string `json:"type"` // "enabled" or "disabled"
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []message     `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
	Thinking    thinkingParam `json:"thinking"`
	Tools       []chatTool    `json:"tools,omitempty"`
}

// chatTool enables a server-side DeepSeek tool (web search). The client does
// not execute anything: searching happens on DeepSeek's side.
type chatTool struct {
	Type string `json:"type"`
}

// webSearchMaxTokens is used when web search is enabled: a plain 16-token
// limit truncates answers that come with explanations or citations.
const webSearchMaxTokens = 256

// thinkingMaxTokens is used when thinking (reasoning) is enabled: reasoning
// tokens count towards max_tokens, so a small limit leaves content empty.
const thinkingMaxTokens = 4096

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Annotations      []any  `json:"annotations"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		ServerToolUse struct {
			ToolSearch      int `json:"tool_search"`
			ToolFetch       int `json:"tool_fetch"`
			ToolImageSearch int `json:"tool_image_search"`
		} `json:"server_tool_use"`
	} `json:"usage"`
}

func (c *DeepSeekClient) AskText(ctx context.Context, text string) (string, error) {
	return c.chat(ctx, []message{
		{Role: "system", Content: c.cfg.SystemPrompt},
		{Role: "user", Content: text},
	})
}

func (c *DeepSeekClient) AskImage(ctx context.Context, png []byte) (string, error) {
	// OpenAI-compatible vision input.
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	content := []map[string]any{
		{"type": "text", "text": "Ответь на вопрос по этому изображению."},
		{"type": "image_url", "image_url": map[string]any{"url": dataURI}},
	}
	return c.chat(ctx, []message{
		{Role: "system", Content: c.cfg.SystemPrompt},
		{Role: "user", Content: content},
	})
}

func (c *DeepSeekClient) chat(ctx context.Context, msgs []message) (string, error) {
	thinking := "disabled"
	if c.cfg.Thinking {
		thinking = "enabled"
	}

	payload := chatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		Temperature: c.cfg.Temperature,
		MaxTokens:   c.cfg.MaxTokens,
		Stream:      false,
		Thinking:    thinkingParam{Type: thinking},
	}
	if c.cfg.WebSearch {
		payload.Tools = []chatTool{{Type: "deepseek_web_search"}}
		if payload.MaxTokens < webSearchMaxTokens {
			payload.MaxTokens = webSearchMaxTokens
		}
	}
	if c.cfg.Thinking && payload.MaxTokens < thinkingMaxTokens {
		payload.MaxTokens = thinkingMaxTokens
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("unexpected API response: %s", truncate(string(body), 300))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("no choices in response: %s", truncate(string(body), 300))
	}

	choice := parsed.Choices[0]
	if c.cfg.Thinking {
		log.Printf("thinking: reasoning_len=%d finish=%s", len(choice.Message.ReasoningContent), choice.FinishReason)
	}
	if c.cfg.WebSearch {
		u := parsed.Usage.ServerToolUse
		log.Printf("web search: queries=%d fetches=%d image_search=%d citations=%d",
			u.ToolSearch, u.ToolFetch, u.ToolImageSearch, len(choice.Message.Annotations))
	}
	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		return "", fmt.Errorf(
			"empty content (finish_reason=%q, reasoning_len=%d, thinking=%s)",
			choice.FinishReason, len(choice.Message.ReasoningContent), thinking,
		)
	}
	return content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
