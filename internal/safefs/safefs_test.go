package safefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRejects(t *testing.T) {
	bad := []string{
		"", "..\\x", "../x", "a/../../b", "/etc/passwd", "\\\\server\\share\\f",
		"C:\\Windows\\x", "file.txt:stream", "CON", "dir/NUL.txt", "a\x00b", "a|b", "a?b",
	}
	for _, s := range bad {
		if _, err := Normalize(s); err == nil {
			t.Errorf("Normalize(%q) should fail", s)
		}
	}
}

func TestNormalizeAccepts(t *testing.T) {
	good := []string{"IMAGES", "IMAGESloja\\CPU0.bmp", "a/b/c.ltg", "./x.ltg", "Wally Mart.ltg"}
	for _, s := range good {
		if _, err := Normalize(s); err != nil {
			t.Errorf("Normalize(%q): %v", s, err)
		}
	}
}

func TestResolveConfined(t *testing.T) {
	dir := t.TempDir()
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := root.Resolve("sub\\file.ltg")
	if err != nil || filepath.Dir(filepath.Dir(p)) != root.Dir() {
		t.Fatalf("p=%q err=%v", p, err)
	}
	if _, err := root.Resolve("..\\outside"); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("want ErrOutsideRoot got %v", err)
	}
}

func TestResolveFoldCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "IMAGES"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "IMAGES", "CPU0.BMP"), []byte("x"), 0o644)
	root, _ := NewRoot(dir)
	p, err := root.ResolveFold("images\\cpu0.bmp")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "CPU0.BMP" {
		t.Fatalf("got %q", p)
	}
	if _, err := root.ResolveFold("images\\nope.bmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want ErrNotExist got %v", err)
	}
}

func TestSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	root, _ := NewRoot(dir)
	if _, err := root.Resolve("link"); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("symlink escape not blocked: %v", err)
	}
}

func TestReadLimited(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.bin")
	_ = os.WriteFile(p, make([]byte, 100), 0o644)
	if _, err := ReadLimited(p, 50); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge got %v", err)
	}
	if b, err := ReadLimited(p, 100); err != nil || len(b) != 100 {
		t.Fatalf("len=%d err=%v", len(b), err)
	}
	if _, err := ReadLimited(filepath.Dir(p), 100); err == nil {
		t.Fatal("directory should fail")
	}
}

func TestWriteAtomicKeepsBackup(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.ltg")
	if err := WriteAtomic(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(p)
	bak, _ := os.ReadFile(p + ".bak")
	if string(cur) != "two" || string(bak) != "one" {
		t.Fatalf("cur=%q bak=%q", cur, bak)
	}
}
