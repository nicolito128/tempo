package player

import (
	"fmt"
)

type AudioFile struct {
	name string
	path string
}

func NewAudioFile(path string) AudioFile {
	return AudioFile{}
}

func (a *AudioFile) FilterValue() string {
	return a.name
}

func (a *AudioFile) Name() string {
	return a.name
}

func (a *AudioFile) SetName(name string) {
	if name == "" {
		return
	}
	a.name = name
}

func (a *AudioFile) Path() string {
	return a.path
}

func (a *AudioFile) SetPath(path string) {
	a.path = path
}

func (a *AudioFile) String() string {
	if a == nil {
		return "nil"
	}
	return fmt.Sprintf("%#v", a)
}
