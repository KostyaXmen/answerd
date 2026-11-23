package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTakeOldestInboxImage(t *testing.T) {
	dir := t.TempDir()

	if item, err := takeOldestInboxImage(dir); err != nil || item != nil {
		t.Fatalf("empty inbox: item=%v err=%v", item, err)
	}
	if _, err := takeOldestInboxImage(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected an error for a missing inbox directory")
	}

	older := filepath.Join(dir, "shot-20260101-000000.000000000.png")
	newer := filepath.Join(dir, "shot-20260102-000000.000000000.png")
	tmp := filepath.Join(dir, "shot-20260103-000000.000000000.png.tmp")
	for path, data := range map[string]string{older: "AAA", newer: "BBB", tmp: "CCC"} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	item, err := takeOldestInboxImage(dir)
	if err != nil {
		t.Fatalf("takeOldestInboxImage: %v", err)
	}
	if item == nil || item.Kind != "image" || string(item.Data) != "AAA" || item.Hash == "" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if _, err := os.Stat(older); !os.IsNotExist(err) {
		t.Fatal("oldest screenshot was not removed")
	}
	if _, err := os.Stat(newer); err != nil {
		t.Fatal("newer screenshot should stay in the inbox")
	}

	item, err = takeOldestInboxImage(dir)
	if err != nil || item == nil || string(item.Data) != "BBB" {
		t.Fatalf("second call: item=%v err=%v", item, err)
	}
}
