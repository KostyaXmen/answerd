package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type ClipboardItem struct {
	Kind string // "text" or "image"
	Data []byte // text bytes or PNG bytes
	Hash string
}

func (c *ClipboardItem) Text() string {
	return string(c.Data)
}

func dataHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func runClip(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// timedOut reports whether err is just the command being killed because our
// read deadline expired. Those are expected while another app (e.g. spectacle)
// holds the clipboard and must not be logged as failures.
func timedOut(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return true
		}
	}
	return false
}

// noSelection reports wl-paste's "nothing is copied" exit code. This happens
// for a moment after an app that owned the clipboard exits (e.g. spectacle).
func noSelection(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 1
}

// plainTextType picks the plain-text MIME offered by the clipboard owner.
// Spectacle/Klipper leave application/x-kde-onlyReplaceEmpty around; treating
// arbitrary types as text used to feed junk like "1" to the model.
func plainTextType(types map[string]bool) (string, bool) {
	if types["text/plain"] {
		return "text/plain", true
	}
	if types["text/plain;charset=utf-8"] {
		return "text/plain;charset=utf-8", true
	}
	for t := range types {
		if strings.HasPrefix(t, "text/plain;") {
			return t, true
		}
	}
	return "", false
}

func readClipboard(parent context.Context) (*ClipboardItem, error) {
	// Bound the read so an empty clipboard never blocks the poller.
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()

	typesRaw, err := runClip(ctx, "wl-paste", "--list-types")
	if err != nil {
		if timedOut(ctx, err) || noSelection(err) {
			return nil, nil // timed out or empty clipboard
		}
		return nil, err
	}

	types := make(map[string]bool)
	for _, line := range strings.Split(string(typesRaw), "\n") {
		types[strings.TrimSpace(line)] = true
	}

	if types["image/png"] {
		png, err := runClip(ctx, "wl-paste", "--type", "image/png")
		if err != nil {
			if timedOut(ctx, err) || noSelection(err) {
				return nil, nil
			}
			return nil, err
		}
		if len(png) > 0 {
			return &ClipboardItem{Kind: "image", Data: png, Hash: dataHash(png)}, nil
		}
		return nil, nil
	}

	mime, ok := plainTextType(types)
	if !ok {
		return nil, nil
	}
	text, err := runClip(ctx, "wl-paste", "--no-newline", "--type", mime)
	if err != nil {
		if timedOut(ctx, err) || noSelection(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(text)) > 0 {
		return &ClipboardItem{Kind: "text", Data: text, Hash: dataHash(text)}, nil
	}
	return nil, nil
}
