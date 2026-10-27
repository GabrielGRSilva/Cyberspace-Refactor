// Package app contains the application services that sit between the UI and
// the engine: workspace discovery, confined file loading and user settings.
package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cyberspace-refactor/internal/assets"
	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/format"
	"cyberspace-refactor/internal/model"
	"cyberspace-refactor/internal/safefs"
)

// Settings are persisted as JSON next to the executable.
type Settings struct {
	Sound      bool `json:"sound"`
	AutoKarma  bool `json:"autoKarma"`
	PoolMode   int  `json:"hackingPoolMode"` // 0 original calibration, 1 SR2 core
	DisplayMod int  `json:"displayModeOverride"`
	LastDeck   string `json:"lastDeck,omitempty"`
	LastDecker string `json:"lastDecker,omitempty"`
	LastLTG    string `json:"lastLTG,omitempty"`
}

// DefaultSettings returns the defaults.
func DefaultSettings() Settings { return Settings{Sound: true, DisplayMod: -1} }

// Workspace is the confined data area (the "assets" folder).
type Workspace struct {
	Base     string // directory containing assets/ (and settings.json)
	Root     *safefs.Root
	Library  *assets.Library
	Settings Settings
}

// Locate finds the assets folder: next to the executable first, then the
// working directory (development).
func Locate() (*Workspace, error) {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, base := range candidates {
		if st, err := os.Stat(filepath.Join(base, "assets")); err == nil && st.IsDir() {
			return Open(base)
		}
	}
	return nil, errors.New("assets folder not found next to the program")
}

// Open builds a Workspace for a base directory containing assets/.
func Open(base string) (*Workspace, error) {
	root, err := safefs.NewRoot(filepath.Join(base, "assets"))
	if err != nil {
		return nil, err
	}
	lib, err := assets.NewLibrary(root.Dir())
	if err != nil {
		return nil, err
	}
	w := &Workspace{Base: base, Root: root, Library: lib, Settings: DefaultSettings()}
	w.LoadSettings()
	return w, nil
}

func (w *Workspace) settingsPath() string { return filepath.Join(w.Base, "settings.json") }

// LoadSettings reads settings.json if present (invalid files are ignored).
func (w *Workspace) LoadSettings() {
	b, err := safefs.ReadLimited(w.settingsPath(), 64<<10)
	if err != nil {
		return
	}
	s := DefaultSettings()
	if json.Unmarshal(b, &s) == nil {
		if s.PoolMode < 0 || s.PoolMode > 1 {
			s.PoolMode = 0
		}
		w.Settings = s
	}
}

// SaveSettings writes settings.json atomically.
func (w *Workspace) SaveSettings() error {
	b, err := json.MarshalIndent(w.Settings, "", "  ")
	if err != nil {
		return err
	}
	return safefs.WriteAtomic(w.settingsPath(), b)
}

// ListFiles returns relative (slash) paths under assets whose extension is in
// exts (lower-case, with dot), up to a bounded count.
func (w *Workspace) ListFiles(exts ...string) []string {
	want := map[string]bool{}
	for _, e := range exts {
		want[strings.ToLower(e)] = true
	}
	var out []string
	_ = filepath.WalkDir(w.Root.Dir(), func(p string, d fs.DirEntry, err error) error {
		if err != nil || len(out) >= 5000 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if want[strings.ToLower(filepath.Ext(p))] {
			if rel, err := filepath.Rel(w.Root.Dir(), p); err == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// LoadMatrix loads an .ltg (or .json) from the workspace.
func (w *Workspace) LoadMatrix(rel string) (*model.Matrix, []string, error) {
	b, err := w.Root.ReadFile(rel, safefs.MaxTextFileBytes)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(strings.ToLower(rel), ".json") {
		m, err := format.MatrixFromJSON(b)
		return m, nil, err
	}
	return format.ParseLTG(b)
}

// LoadDeck loads a .dek (or .json).
func (w *Workspace) LoadDeck(rel string) (*model.Deck, []string, error) {
	b, err := w.Root.ReadFile(rel, safefs.MaxTextFileBytes)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(strings.ToLower(rel), ".json") {
		d, err := format.DeckFromJSON(b)
		return d, nil, err
	}
	return format.ParseDeck(b)
}

// LoadDecker loads a .npc (or .json).
func (w *Workspace) LoadDecker(rel string) (*model.Decker, []string, error) {
	b, err := w.Root.ReadFile(rel, safefs.MaxTextFileBytes)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(strings.ToLower(rel), ".json") {
		d, err := format.DeckerFromJSON(b)
		return d, nil, err
	}
	return format.ParseNPC(b)
}

// Save writes bytes to a workspace-relative path (atomic, with .bak). Only
// the known data extensions are allowed.
func (w *Workspace) Save(rel string, data []byte) error {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".ltg", ".dek", ".npc", ".json":
	default:
		return errors.New("only .ltg, .dek, .npc and .json files can be saved")
	}
	p, err := w.Root.Resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return safefs.WriteAtomic(p, data)
}

// SaveText saves a session history log (.txt or .log) confined to the LOGS/
// subdirectory so existing adventure .txt files in DATA/ can never be
// overwritten.
func (w *Workspace) SaveText(rel string, data []byte) error {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".txt", ".log":
	default:
		return errors.New("only .txt and .log files can be saved as history logs")
	}
	norm, err := safefs.Normalize(rel)
	if err != nil {
		return err
	}
	norm = filepath.ToSlash(norm)
	upper := strings.ToUpper(norm)
	if !strings.HasPrefix(upper, "LOGS/") {
		if strings.Contains(norm, "/") {
			return errors.New("history logs must be saved inside the LOGS/ folder")
		}
		norm = "LOGS/" + norm
	}
	p, err := w.Root.Resolve(norm)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return safefs.WriteAtomic(p, data)
}

// LinkLoader returns an engine.Loader confined to the workspace. LNK targets
// are looked up beside the current LTG first, then in the DATA root.
func (w *Workspace) LinkLoader(currentDir string) engine.Loader {
	return func(name string) (*model.Matrix, error) {
		var tries []string
		if currentDir != "" && currentDir != "." {
			tries = append(tries, currentDir+"/"+name)
		}
		tries = append(tries, "DATA/"+name, name)
		var lastErr error = os.ErrNotExist
		for _, t := range tries {
			m, _, err := w.LoadMatrix(t)
			if err == nil {
				return m, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// ImageDirsFor computes the image search order for a matrix.
func ImageDirsFor(m *model.Matrix, ltgRel string) []string {
	var dirs []string
	if p := strings.TrimSpace(m.ImagePath); p != "" {
		// Legacy absolute Windows paths: keep only the last folder name.
		p = strings.ReplaceAll(p, "\\", "/")
		p = strings.TrimRight(p, "/")
		if i := strings.LastIndex(p, "/"); i >= 0 {
			p = p[i+1:]
		}
		if p != "" && p != ".." && p != "." && !strings.Contains(p, ":") {
			dirs = append(dirs, p)
		}
	}
	if d := filepath.ToSlash(filepath.Dir(ltgRel)); d != "." && d != "" {
		dirs = append(dirs, d+"/IMAGES")
	}
	return append(dirs, "IMAGES")
}
