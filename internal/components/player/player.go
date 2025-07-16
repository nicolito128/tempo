package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

type TickMsg struct {
	tick int
}

// Player : An audio player and bubbletea Model
type Player struct {
	// Streamer audio file
	stream beep.StreamSeekCloser

	// Sample format
	format beep.Format

	// Volume controller
	vol *effects.Volume

	// Ctrl allows to pause the streamer
	ctrl *beep.Ctrl

	hasInit   bool
	isRunning bool

	volume float64

	currentAudio *AudioFile

	duration time.Duration
	length   int

	done chan struct{}

	err error

	mu sync.RWMutex
}

func New(volume int) *Player {
	p := &Player{}
	p.vol = &effects.Volume{}
	p.ctrl = &beep.Ctrl{}
	p.done = make(chan struct{})

	initVol := AbsVolume(volume)
	if volume > 100 || volume < 0 {
		initVol = 0
	}
	p.volume = initVol

	return p
}

func (p *Player) SetAudio(af AudioFile) {
	p.currentAudio = &af
}

func (p *Player) Audio() AudioFile {
	if p.currentAudio != nil {
		return *p.currentAudio
	}
	return AudioFile{}
}

func (p *Player) Init() tea.Cmd {
	p.LoadAudio()
	speaker.Init(p.format.SampleRate, p.format.SampleRate.N(time.Second/10))
	return nil
}

func (p *Player) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !p.hasInit && p.currentAudio != nil {
		return p, p.Play()
	}

	switch msg := msg.(type) {
	case tea.QuitMsg:
		return p, tea.Quit

	case TickMsg:
		if p.isRunning {
			p.length++
		}

		return p, p.tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			p.Close()
			return p, tea.Quit

		case "enter", " ":
			p.StopOrResume()
			return p, nil

		default:
			return p, nil
		}

	default:
		return p, nil
	}
}

func (p *Player) View() string {
	if p.err != nil {
		p.Close()
		return "Error: " + p.err.Error()
	}

	if p.currentAudio != nil {
		var s string
		if !p.isRunning {
			s += "(Paused) "
		} else {
			s += "(Running) "
		}
		s += fmt.Sprintf("File: %s | Time: %ds | Path: %s", p.currentAudio.name, p.length, p.currentAudio.path)
		return s
	}

	return ""
}

func (p *Player) Close() {
	p.currentAudio = nil

	err := p.stream.Close()
	if err != nil {
		p.err = err
	}
}

func (p *Player) Play() tea.Cmd {
	if p.hasInit {
		return nil
	}
	p.hasInit = true

	if p.isRunning {
		return nil
	}
	p.isRunning = true

	speaker.Play(p.ctrl)
	return p.tick()
}

func (p *Player) Resume() {
	if !p.hasInit {
		return
	}
	if p.isRunning {
		return
	}
	p.isRunning = true

	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
}

func (p *Player) Stop() {
	if !p.hasInit {
		return
	}
	if !p.isRunning {
		return
	}
	p.isRunning = false

	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
}

// StopOrResume pauses or unpauses the speaker audio depending if it is running or not.
func (p *Player) StopOrResume() {
	if p.isRunning {
		p.Stop()
	} else {
		p.Resume()
	}
}

func (p *Player) LoadAudio() {
	if p.currentAudio == nil {
		return
	}

	ext := filepath.Ext(p.currentAudio.path)
	if ext == "" {
		p.err = errors.New("invalid file extension")
		return
	}

	file, err := os.Open(p.currentAudio.path)
	if err != nil {
		p.err = err
		return
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	}

	if err != nil {
		p.err = err
		return
	}

	p.stream = streamer
	p.format = format
	p.duration = time.Duration(streamer.Len())
	p.vol.Streamer = streamer
	p.ctrl.Streamer = streamer
}

func (p *Player) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return TickMsg{tick: p.length}
	})
}

// AbsVolume converts human readable form volume (0 - 100) to float64 volume.
func AbsVolume(volume int) float64 {
	return (float64(volume) - 100) / 10
}
