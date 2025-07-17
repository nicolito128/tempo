package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

const VolumeShift float64 = 0.15

type TickMsg struct{}

// Player : An audio player and bubbletea Model
type Player struct {
	// Streamer audio file
	stream beep.StreamSeekCloser

	// Sample format
	format beep.Format

	// Volume controller
	volume *effects.Volume

	// Ctrl allows to pause the streamer
	ctrl *beep.Ctrl

	// hasInit if the audio player is already started
	hasInit bool

	// isRunning if there is an audio being played back
	isRunning bool

	// isQuitting if the user requests to exit the program
	isQuitting bool

	// totalVolume is the volume of the streamer in a human-readable format
	totalVolume int

	// Current audio file being played back
	currentAudio *AudioFile

	// Time duration of the audio file
	duration time.Duration

	// Seconds counter of the audio file being played back
	length int

	err error
}

func New(volume int) *Player {
	p := &Player{}
	if volume > 100 || volume < 0 {
		volume = 0
	}
	p.totalVolume = volume

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
	if p.err != nil {
		p.Close()
		return p, tea.Quit
	}

	if !p.hasInit && p.currentAudio != nil {
		return p, p.Play()
	}

	switch msg := msg.(type) {
	case TickMsg:
		if p.isRunning {
			p.length++
		}

		return p, p.tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			p.Close()
			return p, tea.Quit

		case "enter", " ":
			p.StopOrResume()

		case "+", "up":
			p.IncrementVolume()

		case "-", "down":
			p.DecrementVolume()

		case "m", "M":
			p.ToggleVolume()
		}
	}

	return p, nil
}

func (p *Player) View() string {
	if p.err != nil {
		return "Error: " + p.err.Error()
	}

	if p.isQuitting {
		return ""
	}

	var s string
	if p.currentAudio != nil {
		if p.volume.Silent {
			s += "[ Muted ] "
		}

		if !p.isRunning {
			s += "( Paused ) "
		} else {
			s += "( Playing ) "
		}
		s += fmt.Sprintf("File: %s | Volume: %d | Elapsed: %s | Path: %s",
			p.currentAudio.name,
			p.totalVolume,
			FormatSecondsToString(p.length),
			p.currentAudio.path,
		)
	}

	// help
	s += "\n\nq (quit) | Space (pause/resume) | + / Arrow Up (volume up) | - / Arrow Down (volume down) | m (mute/unmute)\n"

	return s
}

func (p *Player) Close() {
	p.isQuitting = true
	p.currentAudio = nil
	p.duration = 0
	p.length = 0

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

	speaker.Play(p.volume)
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

func (p *Player) DecrementVolume() {
	p.totalVolume -= 5
	if p.totalVolume < 0 {
		p.totalVolume = 0
	}

	if p.totalVolume > 0 {
		p.volume.Volume -= VolumeShift
	}

	if p.totalVolume == 0 {
		p.volume.Silent = true
	} else {
		p.volume.Silent = false
	}
}

func (p *Player) IncrementVolume() {
	p.totalVolume += 5
	if p.totalVolume > 100 {
		p.totalVolume = 100
	}

	if p.totalVolume < 100 {
		p.volume.Volume += VolumeShift
	}

	if p.totalVolume == 0 {
		p.volume.Silent = true
	} else {
		p.volume.Silent = false
	}
}

func (p *Player) MuteVolume() {
	p.volume.Silent = true
}

func (p *Player) UnmuteVolume() {
	p.volume.Silent = false
}

func (p *Player) ToggleVolume() {
	if p.volume.Silent {
		p.UnmuteVolume()
	} else {
		p.MuteVolume()
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

	// Sample audio
	p.stream = streamer
	p.format = format
	p.duration = time.Duration(streamer.Len()).Round(time.Second)
	// Controllers
	p.ctrl = &beep.Ctrl{
		Streamer: streamer,
		Paused:   false,
	}
	p.volume = &effects.Volume{
		Streamer: p.ctrl,
		Base:     1.5,
		Volume:   0,
		Silent:   false,
	}
}

func (p *Player) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}

// AbsVolume converts human readable form volume (0 - 100) to float64 volume.
func AbsVolume(volume int) float64 {
	return (float64(volume) - 100) / 10
}

func FormatSecondsToString(totalSeconds int) string {
	secondsInMinute := 60
	secondsInHour := 60 * 60
	secondsInDay := 60 * 60 * 24

	days := totalSeconds / secondsInDay
	remainder := totalSeconds % secondsInDay

	hours := remainder / secondsInHour
	remainder = remainder % secondsInHour

	minutes := remainder / secondsInMinute
	seconds := remainder % secondsInMinute

	var s string
	if days > 0 {
		s += fmt.Sprintf("%dd ", days)
	}
	if hours > 0 {
		s += fmt.Sprintf("%dh ", hours)
	}
	if minutes > 0 {
		s += fmt.Sprintf("%dm ", minutes)
	}
	if seconds >= 0 {
		s += fmt.Sprintf("%ds", seconds)
	}

	return s
}
