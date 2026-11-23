package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// History appends one JSON line per request and stores input images.
type History struct {
	filePath string
	imageDir string
}

// HistoryRecord is one line in history.jsonl.
type HistoryRecord struct {
	TS        string  `json:"ts"`
	Kind      string  `json:"kind"` // "text" or "image"
	Question  string  `json:"question,omitempty"`
	ImagePath string  `json:"image_path,omitempty"`
	Raw       string  `json:"raw,omitempty"`
	Answer    string  `json:"answer"`
	Status    string  `json:"status"` // "ok" or "error"
	Error     *string `json:"error,omitempty"`
	LatencyMs int64   `json:"latency_ms"`
}

func NewHistory(filePath, imageDir string) (*History, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, err
	}
	if imageDir != "" {
		if err := os.MkdirAll(imageDir, 0o755); err != nil {
			return nil, err
		}
	}
	return &History{filePath: filePath, imageDir: imageDir}, nil
}

// SaveImage writes the PNG to the image dir and returns its path.
func (h *History) SaveImage(png []byte, ts time.Time) (string, error) {
	if h.imageDir == "" {
		return "", nil
	}
	name := ts.Format("20060102-150405.000000000") + ".png"
	path := filepath.Join(h.imageDir, name)
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Append writes a record as a single JSON line.
func (h *History) Append(rec HistoryRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(h.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
