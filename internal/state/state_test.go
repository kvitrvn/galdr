package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	_, err := Load(path)
	if !errors.Is(err, ErrNoState) {
		t.Errorf("Load missing: err = %v, want ErrNoState", err)
	}
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("not json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err == nil {
		t.Error("Load corrupt: expected error, got nil")
	}
	if s.Volume != 100 {
		t.Errorf("Load corrupt: default Volume = %d, want 100", s.Volume)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	want := State{Volume: 75, CurrentPath: "/music/song.mp3"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoad_ClampsVolume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{"volume": 250, "current_path": ""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Volume != 100 {
		t.Errorf("Volume = %d, want 100 after clamp", got.Volume)
	}
}

func TestDefault(t *testing.T) {
	if d := Default(); d.Volume != 100 {
		t.Errorf("Default Volume = %d, want 100", d.Volume)
	}
}
