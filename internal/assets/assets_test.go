package assets

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"golang.org/x/image/bmp"

	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/model"
)

func TestNodeAndICESpriteNaming(t *testing.T) {
	c := NodeSprite(model.CPU, 0, true)
	if len(c) != 2 || c[0] != "CPU0S" || c[1] != "CPU0" {
		t.Fatalf("unexpected CPU0S candidates: %v", c)
	}
	lnk := NodeSprite(model.LNK, 2, false)
	if len(lnk) != 1 || lnk[0] != "SAN2" {
		t.Fatalf("LNK should map to SAN: %v", lnk)
	}
	mat := NodeSprite(model.MAT, 1, false)
	if len(mat) != 1 || mat[0] != "MAT" {
		t.Fatalf("MAT candidates: %v", mat)
	}

	ice := &engine.ICEInst{
		Def:   model.ICE{Class: "Black", Type: "Killer", Rating: 6},
		State: engine.StYelling,
	}
	if s := ICESprite(ice, 2, false, false); len(s) != 2 || s[0] != "ICYELB4" || s[1] != "ICYELB" {
		t.Fatalf("sculpted black killer yelling: %v", s)
	}
	if s := ICESprite(ice, 0, true, false); len(s) != 1 || s[0] != "ICYELB" {
		t.Fatalf("standard black killer yelling: %v", s)
	}
	if s := ICESprite(ice, 1, true, false); len(s) != 2 || s[0] != "ICYELB4" {
		t.Fatalf("advanced analysed killer: %v", s)
	}
}

func TestSysOpAndLogSoundMapping(t *testing.T) {
	if s := SysOpSound(engine.OpLockout); s != "SNDSOP0" {
		t.Fatalf("expected SNDSOP0, got %s", s)
	}
	if s := SysOpSound(engine.OpDisconnect); s != "SNDSOPF" {
		t.Fatalf("expected SNDSOPF, got %s", s)
	}
	if s := SoundForLog("ICE wants your identification!"); s != SndLook {
		t.Fatalf("expected SndLook, got %s", s)
	}
	if s := SoundForLog("Passive alert triggered!"); s != SndPassive {
		t.Fatalf("expected SndPassive, got %s", s)
	}
}

func TestLibraryLoadRealAssetsAndWhiteKey(t *testing.T) {
	assetsDir := filepath.Clean("../../assets")
	lib, err := NewLibrary(assetsDir)
	if err != nil {
		t.Skipf("assets directory not present: %v", err)
	}
	img, err := lib.Image([]string{"SAN2"}, false)
	if err != nil || img.Bounds().Dx() == 0 {
		t.Fatalf("expected SAN2.BMP to load, got err=%v", err)
	}
	iceImg, err := lib.Image([]string{"ICLOOK0", "ICLOOK"}, true)
	if err != nil || iceImg.Bounds().Dx() == 0 {
		t.Fatalf("expected ICLOOK sprite to load, got err=%v", err)
	}
	if p := lib.SoundPath(SndBoot); p == "" {
		t.Fatalf("expected SNDBOOT.WAV to resolve")
	}
}

func TestDecodeWhiteKeyAndDimensionBombRejection(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	src.Set(0, 0, color.RGBA{255, 255, 255, 255}) // white -> transparent
	src.Set(1, 0, color.RGBA{10, 200, 30, 255})   // green -> preserved
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, err := decode(buf.Bytes(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := out.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("expected white pixel to become transparent, got alpha=%d", a)
	}
	if _, g, _, a := out.At(1, 0).RGBA(); a == 0 || g == 0 {
		t.Fatalf("expected non-white pixel to remain opaque")
	}

	// Craft a decompression-bomb BMP header (width 10000 x height 10000).
	bomb := append([]byte(nil), buf.Bytes()...)
	binary.LittleEndian.PutUint32(bomb[18:22], 10000)
	binary.LittleEndian.PutUint32(bomb[22:26], 10000)
	if _, err := decode(bomb, false); err == nil {
		t.Fatalf("expected oversized BMP dimensions to be rejected")
	}
}
