// Package safefs provides hardened file access: every path coming from data
// files (LTGImagePath, LNK targets, user selections) is confined to an
// allowed root, and every read is size-limited (OWASP A01/A04, NIST CSF PR.DS).
package safefs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Limits applied across the application.
const (
	MaxTextFileBytes  = 2 << 20  // .ltg/.dek/.npc/.json
	MaxImageFileBytes = 8 << 20  // .bmp/.png
	MaxSoundFileBytes = 16 << 20 // .wav
)

var (
	// ErrOutsideRoot is returned when a path escapes the allowed root.
	ErrOutsideRoot = errors.New("safefs: path escapes allowed root")
	// ErrInvalidName is returned for names that are never legitimate.
	ErrInvalidName = errors.New("safefs: invalid file name")
	// ErrTooLarge is returned when a file exceeds the size limit.
	ErrTooLarge = errors.New("safefs: file too large")
)

var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// Normalize converts legacy Windows separators and rejects dangerous input.
func Normalize(rel string) (string, error) {
	if rel == "" || len(rel) > 260 {
		return "", ErrInvalidName
	}
	if strings.ContainsRune(rel, 0) {
		return "", ErrInvalidName
	}
	s := strings.ReplaceAll(rel, "\\", "/")
	// UNC, drive letters, alternate data streams and absolute paths.
	if strings.HasPrefix(s, "/") || strings.Contains(s, ":") {
		return "", ErrOutsideRoot
	}
	for _, part := range strings.Split(s, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", ErrOutsideRoot
		}
		base := strings.ToUpper(strings.TrimRight(part, ". "))
		if i := strings.IndexByte(base, '.'); i >= 0 {
			base = base[:i]
		}
		if reservedNames[base] {
			return "", ErrInvalidName
		}
		for _, r := range part {
			if r < 0x20 || strings.ContainsRune(`<>"|?*`, r) {
				return "", ErrInvalidName
			}
		}
	}
	return filepath.FromSlash(strings.TrimPrefix(s, "./")), nil
}

// Root confines file access to a directory tree.
type Root struct {
	dir string
}

// NewRoot validates and returns a Root for dir.
func NewRoot(dir string) (*Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("safefs: %q is not a directory", dir)
	}
	return &Root{dir: abs}, nil
}

// Dir returns the absolute root directory.
func (r *Root) Dir() string { return r.dir }

// Resolve joins rel to the root, guaranteeing the result stays inside it,
// including after symlink resolution of existing components.
func (r *Root) Resolve(rel string) (string, error) {
	n, err := Normalize(rel)
	if err != nil {
		return "", err
	}
	full := filepath.Join(r.dir, n)
	if !within(r.dir, full) {
		return "", ErrOutsideRoot
	}
	if real, err := filepath.EvalSymlinks(full); err == nil {
		realRoot, rerr := filepath.EvalSymlinks(r.dir)
		if rerr != nil {
			realRoot = r.dir
		}
		if !within(realRoot, real) {
			return "", ErrOutsideRoot
		}
	}
	return full, nil
}

// ResolveFold is like Resolve but matches each path element
// case-insensitively against what exists (legacy data was case-agnostic).
func (r *Root) ResolveFold(rel string) (string, error) {
	n, err := Normalize(rel)
	if err != nil {
		return "", err
	}
	cur := r.dir
	for _, part := range strings.Split(filepath.ToSlash(n), "/") {
		if part == "" {
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", err
		}
		found := ""
		for _, e := range entries {
			if strings.EqualFold(e.Name(), part) {
				found = e.Name()
				break
			}
		}
		if found == "" {
			return "", os.ErrNotExist
		}
		cur = filepath.Join(cur, found)
	}
	return r.Resolve(mustRel(r.dir, cur))
}

func mustRel(base, p string) string {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// ReadFile reads a file inside the root, enforcing max bytes.
func (r *Root) ReadFile(rel string, max int64) ([]byte, error) {
	p, err := r.ResolveFold(rel)
	if err != nil {
		return nil, err
	}
	return ReadLimited(p, max)
}

// ReadLimited reads an absolute path (already trusted/selected by the user)
// refusing files larger than max bytes.
func ReadLimited(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("safefs: %q is a directory", path)
	}
	if st.Size() > max {
		return nil, ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrTooLarge
	}
	return data, nil
}

// WriteAtomic writes data to path via a temp file + rename, keeping a .bak of
// any previous file (NIST CSF RC.RP: recoverability).
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cyber-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path + ".bak")
		if err := os.Rename(path, path+".bak"); err != nil {
			cleanup()
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
