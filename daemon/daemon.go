package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func RunDaemon(ctx context.Context, cfg Config) error {
	state, err := NewState(cfg.StateFile)
	if err != nil {
		return err
	}
	history, err := NewHistory(cfg.HistoryFile, cfg.ImageDir)
	if err != nil {
		return err
	}
	client := NewDeepSeekClient(cfg)

	items := make(chan *ClipboardItem, 1)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		pollLoop(ctx, cfg, items)
	}()
	go func() {
		defer wg.Done()
		workLoop(ctx, client, state, history, items)
	}()
	go func() {
		defer wg.Done()
		inboxLoop(ctx, cfg, items)
	}()

	log.Printf("daemon started: model=%s poll=%dms", cfg.Model, cfg.PollIntervalMs)
	log.Printf("  state file:   %s", cfg.StateFile)
	log.Printf("  history file: %s", cfg.HistoryFile)
	log.Printf("  image dir:    %s", cfg.ImageDir)

	<-ctx.Done()
	log.Printf("shutting down")
	wg.Wait()
	log.Printf("stopped")
	return nil
}

func pollLoop(ctx context.Context, cfg Config, items chan *ClipboardItem) {
	ticker := time.NewTicker(time.Duration(cfg.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	lastHash := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			item, err := readClipboard(ctx)
			if err != nil {
				log.Printf("clipboard read failed: %v", err)
				continue
			}
			if item == nil || item.Hash == lastHash {
				continue
			}
			lastHash = item.Hash
			if item.Kind == "text" && looksLikeFilePath(item.Text()) {
				log.Printf("ignoring clipboard text that looks like a file path")
				continue
			}
			// Ignore tiny clipboard text (e.g. a stray "1") so it does not
			// overwrite a real answer with noise.
			if item.Kind == "text" && len(strings.TrimSpace(item.Text())) < cfg.MinTextLength {
				log.Printf("ignoring short clipboard text (%d chars): %q", len(strings.TrimSpace(item.Text())), summarize(item.Text()))
				continue
			}
			// Latest wins: drop any pending item if the buffer is full.
			select {
			case items <- item:
			default:
				<-items
				items <- item
			}
			log.Printf("clipboard changed: %s (%d bytes, hash %.8s)", item.Kind, len(item.Data), item.Hash)
		}
	}
}

func workLoop(ctx context.Context, client *DeepSeekClient, state *State, history *History, items <-chan *ClipboardItem) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-items:
			process(ctx, client, state, history, item)
		}
	}
}

func process(ctx context.Context, client *DeepSeekClient, state *State, history *History, item *ClipboardItem) {
	start := time.Now()
	state.Write("", "processing", nil)

	rec := HistoryRecord{
		TS:     start.Format(time.RFC3339),
		Kind:   item.Kind,
		Status: "ok",
	}

	switch item.Kind {
	case "text":
		rec.Question = redactSecrets(item.Text())
		log.Printf("question (text, %d bytes): %s", len(item.Data), summarize(rec.Question))
	case "image":
		if path, err := history.SaveImage(item.Data, start); err != nil {
			log.Printf("failed to save image: %v", err)
		} else if path != "" {
			rec.ImagePath = path
			log.Printf("question (image, %d bytes) saved to: %s", len(item.Data), path)
		}
	}

	var raw string
	var err error
	if item.Kind == "text" {
		raw, err = client.AskText(ctx, rec.Question)
	} else {
		raw, err = client.AskImage(ctx, item.Data)
	}
	rec.LatencyMs = time.Since(start).Milliseconds()
	rec.Raw = redactSecrets(raw)

	if err != nil {
		if ctx.Err() != nil {
			return // shutting down, do not overwrite state
		}
		log.Printf("request failed after %dms: %v", rec.LatencyMs, err)
		rec.Status = "error"
		msg := err.Error()
		rec.Error = &msg
		appendHistory(history, rec)
		state.Write("", "error", err)
		return
	}

	rec.Answer = normalize(rec.Raw)

	if rec.Answer != "" {
		log.Printf("answer: %s (raw: %q, %dms)", rec.Answer, rec.Raw, rec.LatencyMs)
		state.Write(rec.Answer, "ok", nil)
	} else {
		log.Printf("no digits in response: %q (%dms)", rec.Raw, rec.LatencyMs)
		rec.Status = "error"
		msg := "no digits in response: " + rec.Raw
		rec.Error = &msg
		state.Write("", "error", fmt.Errorf("no digits in response: %q", rec.Raw))
	}
	appendHistory(history, rec)
}

func inboxLoop(ctx context.Context, cfg Config, items chan *ClipboardItem) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			item, err := takeOldestInboxImage(cfg.InboxDir)
			if err != nil || item == nil {
				continue
			}
			select {
			case items <- item:
			case <-ctx.Done():
				return
			}
			log.Printf("inbox: image (%d bytes, hash %.8s)", len(item.Data), item.Hash)
		}
	}
}

// takeOldestInboxImage returns the oldest screenshot produced by `answerd
// screenshot` and removes it from the inbox.
func takeOldestInboxImage(dir string) (*ClipboardItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "shot-") || !strings.HasSuffix(name, ".png") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, nil
	}
	sort.Strings(names)

	path := filepath.Join(dir, names[0])
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return &ClipboardItem{Kind: "image", Data: data, Hash: dataHash(data)}, nil
}

func appendHistory(h *History, rec HistoryRecord) {
	if err := h.Append(rec); err != nil {
		log.Printf("failed to write history: %v", err)
	}
}

func summarize(s string) string {
	const max = 200
	s = redactSecrets(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
