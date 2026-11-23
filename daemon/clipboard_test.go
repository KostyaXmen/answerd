package main

import "testing"

func TestPlainTextType(t *testing.T) {
	cases := []struct {
		name  string
		types map[string]bool
		want  string
		ok    bool
	}{
		{
			name:  "klipper onlyReplaceEmpty marker is not text",
			types: map[string]bool{"application/x-kde-onlyReplaceEmpty": true},
			want:  "",
			ok:    false,
		},
		{
			name:  "plain text",
			types: map[string]bool{"text/plain": true, "text/html": true},
			want:  "text/plain",
			ok:    true,
		},
		{
			name:  "utf-8 text",
			types: map[string]bool{"text/plain;charset=utf-8": true},
			want:  "text/plain;charset=utf-8",
			ok:    true,
		},
		{
			name:  "image only",
			types: map[string]bool{"image/png": true},
			want:  "",
			ok:    false,
		},
		{
			name:  "empty",
			types: map[string]bool{},
			want:  "",
			ok:    false,
		},
	}
	for _, c := range cases {
		got, ok := plainTextType(c.types)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: plainTextType() = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}
