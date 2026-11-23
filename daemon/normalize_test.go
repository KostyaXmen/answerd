package main

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1", "01"},
		{"3", "03"},
		{"5", "05"},
		{"9", "09"},
		{"1,5", "15"},
		{"6", "06"},
		{"Ответ: 1/3", "13"},
		{"варианты 2 и 4", "24"},
		{"1,3", "13"},
		{"3/1", "13"},
		{"ничего", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
