package app

import (
	"os"
	"path/filepath"
	"testing"

	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
)

func TestWorkspaceOpenLoadAndConfinement(t *testing.T) {
	tmp := t.TempDir()
	assetsDir := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(filepath.Join(assetsDir, "DATA"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(assetsDir, "IMAGES"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(assetsDir, "SOUNDS"), 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}

	// Save & load settings.
	ws.Settings.AutoKarma = true
	ws.Settings.PoolMode = 1
	if err := ws.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	ws.Settings = DefaultSettings()
	ws.LoadSettings()
	if !ws.Settings.AutoKarma || ws.Settings.PoolMode != 1 {
		t.Fatalf("settings round-trip failed: %+v", ws.Settings)
	}

	// Save & load Matrix (.ltg and .json).
	m := model.NewMatrix()
	m.Description = "Test Grid"
	m.Nodes[0] = &model.Node{ID: 0, Kind: model.SAN, Color: "Blue", Rating: 3, X: 0, Y: 0}
	if err := ws.Save("DATA/test.ltg", format.WriteLTG(m)); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := ws.LoadMatrix("DATA/test.ltg")
	if err != nil || loaded.Description != "Test Grid" {
		t.Fatalf("LoadMatrix failed: %v", err)
	}

	jb, err := format.MatrixToJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Save("DATA/test.json", jb); err != nil {
		t.Fatal(err)
	}
	if m2, _, err := ws.LoadMatrix("DATA/test.json"); err != nil || m2.Description != "Test Grid" {
		t.Fatalf("LoadMatrix JSON failed: %v", err)
	}

	// Reject arbitrary extensions in Save.
	if err := ws.Save("DATA/evil.exe", []byte("nope")); err == nil {
		t.Fatalf("expected .exe save to be rejected")
	}
	if err := ws.Save("DATA/adventure.txt", []byte("nope")); err == nil {
		t.Fatalf("expected .txt save via Save() to be rejected")
	}

	// SaveText must confine logs to LOGS/ and reject writing to DATA/.
	if err := ws.SaveText("DATA/adventure.txt", []byte("overwrite attempt")); err == nil {
		t.Fatalf("expected SaveText into DATA/ to be rejected")
	}
	if err := ws.SaveText("run1.txt", []byte("log line\r\n")); err != nil {
		t.Fatalf("expected SaveText('run1.txt') to succeed under LOGS/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "LOGS", "run1.txt")); err != nil {
		t.Fatalf("expected LOGS/run1.txt to exist: %v", err)
	}
}
