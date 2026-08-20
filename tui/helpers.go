package tui

import (
	"fmt"
	"path/filepath"
)

func hasAudioExt(filename string) bool {
	ext := filepath.Ext(filename)
	switch ext {
	case ".mp3", ".wav", ".flac", ".ogg":
		return true
	default:
		return false
	}
}

func reverseCutString(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}

	lastNRunes := runes[len(runes)-n:]
	return fmt.Sprintf("...%s", string(lastNRunes))
}
