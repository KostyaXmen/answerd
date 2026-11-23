package main

import (
	"regexp"
	"strings"
)

var secretRe = regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`)

// redactSecrets masks DeepSeek-style API keys in text that may be logged or
// stored in history.
func redactSecrets(s string) string {
	if s == "" {
		return s
	}
	return secretRe.ReplaceAllString(s, "sk-***")
}

// looksLikeFilePath reports whether clipboard text is a file URL (for example,
// when a file is copied in Dolphin). Such text is not a question.
func looksLikeFilePath(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "file://")
}
