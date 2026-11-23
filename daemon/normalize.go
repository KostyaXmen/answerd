package main

import (
	"regexp"
	"sort"
	"strings"
)

var answerDigitRe = regexp.MustCompile(`[1-9]`)

// extractDigits returns the unique answer digits (1-9) in ascending order.
func extractDigits(raw string) []int {
	if raw == "" {
		return nil
	}
	matches := answerDigitRe.FindAllString(raw, -1)
	seen := make(map[int]struct{}, len(matches))
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		n := int(m[0] - '0')
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// formatAnswer renders digits for the widget: [1] -> "01", [1, 3] -> "13".
func formatAnswer(digits []int) string {
	if len(digits) == 0 {
		return ""
	}
	if len(digits) == 1 {
		return "0" + string(rune('0'+digits[0]))
	}
	var b strings.Builder
	for _, d := range digits {
		b.WriteByte(byte('0' + d))
	}
	return b.String()
}

func normalize(raw string) string {
	return formatAnswer(extractDigits(raw))
}
