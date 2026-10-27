package assets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/bmp"

	"cyberspace-refactor/internal/safefs"
)

// Maximum accepted image dimensions (anti decompression-bomb, OWASP A04).
const maxDim = 2048

// Library loads images and sounds from a confined directory tree.
type Library struct {
	root      *safefs.Root
	imageDirs []string
	soundDir  string

	mu    sync.Mutex
	cache map[string]*image.RGBA
}

// NewLibrary creates a Library rooted at dir. imageDirs are searched in order
// (relative to dir), e.g. the LTG's own image folder, then "IMAGES".
func NewLibrary(dir string) (*Library, error) {
	r, err := safefs.NewRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Library{root: r, imageDirs: []string{"IMAGES"}, soundDir: "SOUNDS", cache: map[string]*image.RGBA{}}, nil
}

// SetImageDirs sets the ordered list of image folders (relative, validated on use).
func (l *Library) SetImageDirs(dirs ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.imageDirs = dirs
	l.cache = map[string]*image.RGBA{}
}

// ErrNotFound means no candidate image/sound exists.
var ErrNotFound = errors.New("assets: not found")

func bmpDims(b []byte) (w, h int, err error) {
	if len(b) < 26 || b[0] != 'B' || b[1] != 'M' {
		return 0, 0, errors.New("assets: not a BMP")
	}
	w = int(int32(binary.LittleEndian.Uint32(b[18:22])))
	h = int(int32(binary.LittleEndian.Uint32(b[22:26])))
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 || w > maxDim || h > maxDim {
		return 0, 0, errors.New("assets: BMP dimensions out of range")
	}
	return
}

// Image returns the first existing candidate (name without extension). When
// whiteKey is true, pure white (#FFFFFF) pixels become transparent, as the
// original did for ICE sprites.
func (l *Library) Image(candidates []string, whiteKey bool) (*image.RGBA, error) {
	l.mu.Lock()
	dirs := append([]string(nil), l.imageDirs...)
	l.mu.Unlock()
	for _, name := range candidates {
		for _, d := range dirs {
			rel := d + "/" + name + ".bmp"
			key := strings.ToLower(rel)
			if whiteKey {
				key += "|k"
			}
			l.mu.Lock()
			img, ok := l.cache[key]
			l.mu.Unlock()
			if ok {
				return img, nil
			}
			data, err := l.root.ReadFile(rel, safefs.MaxImageFileBytes)
			if err != nil {
				continue
			}
			rgba, err := decode(data, whiteKey)
			if err != nil {
				continue
			}
			l.mu.Lock()
			l.cache[key] = rgba
			l.mu.Unlock()
			return rgba, nil
		}
	}
	return nil, ErrNotFound
}

func decode(data []byte, whiteKey bool) (*image.RGBA, error) {
	if _, _, err := bmpDims(data); err != nil {
		return nil, err
	}
	src, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	if whiteKey {
		for i := 0; i+3 < len(out.Pix); i += 4 {
			if out.Pix[i] == 255 && out.Pix[i+1] == 255 && out.Pix[i+2] == 255 {
				out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = 0, 0, 0, 0
			}
		}
	}
	return out, nil
}

// Placeholder returns a simple generated image used when a sprite is missing.
func Placeholder(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

// SoundPath resolves a sound event (file name without extension) to an
// existing confined path, or "" if absent.
func (l *Library) SoundPath(event string) string {
	if event == "" {
		return ""
	}
	p, err := l.root.ResolveFold(l.soundDir + "/" + event + ".wav")
	if err != nil {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() || st.Size() > safefs.MaxSoundFileBytes {
		return ""
	}
	return p
}

// Play plays a sound event asynchronously if sound is enabled and available.
func (l *Library) Play(event string) {
	if p := l.SoundPath(event); p != "" {
		playFile(p)
	}
}
