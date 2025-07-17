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

	// Buffer format for stream
	format beep.Format

	// Volume controller
	volume *effects.Volume

	// Ctrl allows to pause the streamer
	ctrl *beep.Ctrl

	// hasInit if player is already started and audio loaded as streamer
	hasInit bool

	// running if audio is being played
	running bool

	// completed if the current audio has finished
	completed bool

	// quitting if the user requests to exit the program or if something goes wrong
	quitting bool

	// totalVolume is the volume of the streamer in a human-readable format (from 0 to 100)
	totalVolume int

	// Current audio file being played back
	currentAudio *AudioFile

	// Time duration of the audio file
	duration time.Duration

	// Elapsed in seconds of the audio file being played
	elapsed int

	// error to handle
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
		return p, p.Quit()
	}

	if !p.hasInit && p.currentAudio != nil {
		return p, p.Play()
	}

	switch msg := msg.(type) {
	case TickMsg:
		if !p.completed && p.running {
			p.elapsed++
		}
		return p, p.tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return p, p.Quit()

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

	if p.quitting {
		return ""
	}

	var s string
	if p.currentAudio != nil {
		if p.volume.Silent {
			s += "[ Muted ] "
		}

		if p.completed {
			s += "( Finished ) "
		} else {
			if p.running {
				s += "( Playing ) "
			} else {
				s += "( Paused ) "
			}
		}
		s += fmt.Sprintf("File: %s | Volume: %d | Elapsed: %s | Path: %s",
			p.currentAudio.name,
			p.totalVolume,
			FormatSecondsToString(p.elapsed),
			p.currentAudio.path,
		)
	}

	// help
	s += "\n\nq (quit) | Space (pause/resume) | + / Arrow Up (volume up) | - / Arrow Down (volume down) | m (mute/unmute)\n"

	return s
}

func (p *Player) Reset() {
	p.currentAudio = nil
	p.hasInit = false
	p.running = false
	p.completed = false
	p.quitting = false
	p.totalVolume = 50
	p.duration = 0
	p.elapsed = 0
}

func (p *Player) Close() error {
	p.running = false
	err := p.stream.Close()
	if err != nil {
		p.err = err
	}
	return err
}

func (p *Player) Quit() tea.Cmd {
	p.quitting = true
	p.Close()
	return tea.Quit
}

func (p *Player) Play() tea.Cmd {
	if p.hasInit {
		return nil
	}
	p.hasInit = true

	// If we are already playing a song, then do nothing
	if p.running {
		return nil
	}
	// Otherwise, start playing the song
	p.running = true

	done := make(chan struct{})
	speaker.Play(beep.Seq(p.volume, beep.Callback(func() {
		done <- struct{}{}
	})))

	go func() {
		<-done
		p.completed = true
	}()

	return p.tick()
}

func (p *Player) Resume() {
	if !p.hasInit {
		return
	}
	if p.completed || p.running {
		return
	}
	p.running = true

	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
}

func (p *Player) Stop() {
	if !p.hasInit {
		return
	}
	if p.completed || !p.running {
		return
	}
	p.running = false

	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
}

// StopOrResume pauses or unpauses the speaker audio depending if it is running or not.
func (p *Player) StopOrResume() {
	if p.running {
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
