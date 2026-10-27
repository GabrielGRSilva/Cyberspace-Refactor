//go:build !windows

package assets

// playFile is a no-op outside Windows (the product targets Windows 10/11).
func playFile(string) {}
