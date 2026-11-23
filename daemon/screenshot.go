package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// captureToInbox takes a screenshot into cfg.InboxDir and marks the state as
// processing right away, so the widget shows "00" during the capture.
func captureToInbox(ctx context.Context, cfg Config) error {
	state, err := NewState(cfg.StateFile)
	if err == nil {
		_ = state.Write("", "processing", nil)
	}

	if err := os.MkdirAll(cfg.InboxDir, 0o755); err != nil {
		return fmt.Errorf("create inbox: %w", err)
	}
	tmpDir := filepath.Join(cfg.InboxDir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create inbox tmp dir: %w", err)
	}

	final := filepath.Join(cfg.InboxDir, "shot-"+time.Now().Format("20060102-150405.000000000")+".png")
	tmp := filepath.Join(tmpDir, filepath.Base(final))
	_ = os.Remove(tmp)

	cmd := exec.CommandContext(ctx, "spectacle", "-f", "-b", "-n", "-o", tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		failErr := fmt.Errorf("spectacle: %v", err)
		_ = state.Write("", "error", failErr)
		return fmt.Errorf("spectacle failed: %w: %s", err, string(out))
	}

	info, statErr := os.Stat(tmp)
	if statErr != nil || info.Size() == 0 {
		failErr := fmt.Errorf("spectacle produced no file")
		_ = state.Write("", "error", failErr)
		return failErr
	}

	if err := os.Rename(tmp, final); err != nil {
		_ = state.Write("", "error", err)
		return fmt.Errorf("move screenshot: %w", err)
	}
	return nil
}
