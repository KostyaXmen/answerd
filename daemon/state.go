package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	path string
}

type statePayload struct {
	Answer string  `json:"answer"`
	Status string  `json:"status"`
	Error  *string `json:"error,omitempty"`
	TS     float64 `json:"ts"`
}

func NewState(path string) (*State, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &State{path: path}, nil
}

func (s *State) Write(answer, status string, err error) error {
	payload := statePayload{
		Answer: answer,
		Status: status,
		TS:     float64(time.Now().UnixNano()) / 1e9,
	}
	if err != nil {
		msg := err.Error()
		payload.Error = &msg
	}

	data, jerr := json.Marshal(payload)
	if jerr != nil {
		return jerr
	}

	tmp := s.path + ".tmp"
	if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
		return werr
	}
	return os.Rename(tmp, s.path)
}
