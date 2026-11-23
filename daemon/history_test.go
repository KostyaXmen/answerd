package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryAppendAndSaveImage(t *testing.T) {
	dir := t.TempDir()
	h, err := NewHistory(filepath.Join(dir, "history.jsonl"), filepath.Join(dir, "images"))
	if err != nil {
		t.Fatal(err)
	}

	if err := h.Append(HistoryRecord{TS: "x", Kind: "text", Question: "q", Answer: "01", Status: "ok"}); err != nil {
		t.Fatalf("append text: %v", err)
	}

	imgPath, err := h.SaveImage([]byte("PNGDATA"), time.Unix(0, 0))
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if _, err := os.Stat(imgPath); err != nil {
		t.Fatalf("image not saved: %v", err)
	}

	if err := h.Append(HistoryRecord{TS: "y", Kind: "image", ImagePath: imgPath, Answer: "13", Status: "ok"}); err != nil {
		t.Fatalf("append image: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("history is empty")
	}
}
