//go:build windows

package assets

import (
	"syscall"
	"unsafe"
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	procPlaySound = winmm.NewProc("PlaySoundW")
)

const (
	sndFilename = 0x00020000
	sndAsync    = 0x0001
	sndNoDefault = 0x0002
)

// playFile plays a WAV asynchronously via winmm (no cgo required).
func playFile(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_, _, _ = procPlaySound.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(sndFilename|sndAsync|sndNoDefault))
}
