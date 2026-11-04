package data

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestAggregate24hrsDataIgnoresVolumeMetadata(t *testing.T) {
	dir := t.TempDir()
	fs := afero.NewOsFs()
	old := time.Now().Add(-48 * time.Hour)

	for _, name := range []string{"lost+found", ".snapshots", "nested"} {
		path := filepath.Join(dir, name)
		if err := fs.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fs.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	event := []byte(`{"output":"notice","priority":"Notice","rule":"Terminal shell in container","time":"2021-10-19T17:13:06.279085466Z"}`)
	if err := afero.WriteFile(fs, filepath.Join(dir, "1692197030830418000.json"), event, 0o644); err != nil {
		t.Fatal(err)
	}
	// Sorts after the event file, matching lost+found on a real volume.
	if err := afero.WriteFile(fs, filepath.Join(dir, "zzz.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Aggregate24hrsData(fs, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Rule != "Terminal shell in container" {
		t.Fatalf("got %#v, want the one falco event", got)
	}
	if _, err := fs.Stat(filepath.Join(dir, "lost+found")); err != nil {
		t.Fatalf("lost+found was removed: %v", err)
	}
}

func TestAggregate24hrsDataDeletesOldReportFiles(t *testing.T) {
	dir := t.TempDir()
	fs := afero.NewOsFs()
	path := filepath.Join(dir, "100.json")
	if err := afero.WriteFile(fs, path, []byte(`{"rule":"old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := fs.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	got, err := Aggregate24hrsData(fs, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want no events", got)
	}
	if _, err := fs.Stat(path); err == nil {
		t.Fatal("old report file was kept")
	}
}
