package player

import (
	"fmt"
	"path/filepath"
	"strings"
)

type AudioFile struct {
	name string
	ext  string
	path string
}

func NewAudioFile(path string) AudioFile {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	base = strings.Replace(base, ext, "", 1)
	return AudioFile{name: base, ext: ext, path: path}
}

func (a AudioFile) FilterValue() string {
	return a.name
}

func (a AudioFile) Name() string {
	return a.name
}

func (a *AudioFile) SetName(name string) {
	if name == "" {
		return
	}
	a.name = name
}

func (a AudioFile) Path() string {
	return a.path
}

func (a *AudioFile) SetPath(path string) {
	a.path = path
}

func (a AudioFile) Ext() string {
	return a.ext
}

func (a *AudioFile) String() string {
	if a == nil {
		return "nil"
	}
	return fmt.Sprintf("%#v", a)
}
