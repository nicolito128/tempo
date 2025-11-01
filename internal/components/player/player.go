package player

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
	"github.com/nicolito128/tempo/internal/styles"
)

const (
	// Volume up and down variation (too high shift results in poor audio)
	VolumeShift float64 = 0.16
	// Displays only the last N characters of the path string
	PathCharsLimit int = 32
	// SeekCool is the cooldown time between seek actions
	SeekCooldown time.Duration = 200 * time.Millisecond
)

// TickMsg every second of the played audio
type TickMsg struct{}

// Player : An audio player
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
	elapsed time.Duration

	// error to handle
	err error

	mu sync.RWMutex

	lastSeekTime time.Time
}

var _ tea.Model = (*Player)(nil)

func New(volume int) *Player {
	p := &Player{}
	if volume > 100 {
		volume = 100
	}
	if volume < 0 {
		volume = 0
	}
	p.totalVolume = volume

	return p
}

// SetAudioFile sets the current audio file to be played.
func (p *Player) SetAudioFile(af AudioFile) {
	p.currentAudio = &af
}

// Audio returns the current audio file being played.
func (p *Player) Audio() AudioFile {
	if p.currentAudio != nil {
		return *p.currentAudio
	}
	return AudioFile{}
}

// Init initializes the player, loads the audio file, and sets up the speaker.
func (p *Player) Init() tea.Cmd {
	p.LoadAudio()
	if p.err != nil {
		return p.Quit()
	}
	speaker.Init(p.format.SampleRate, p.format.SampleRate.N(time.Second/10))
	return tea.ClearScreen
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
			p.mu.Lock()
			p.elapsed++
			p.mu.Unlock()
		}
		return p, p.tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return p, p.Quit()

		case "enter", " ":
			if p.completed {
				p.Restart()
			} else {
				p.StopOrResume()
			}

		case "+", "up", "k":
			p.IncrementVolume()

		case "-", "down", "j":
			p.DecrementVolume()

		case "left", "h":
			p.Rewind()

		case "right", "l":
			p.Forward()

		case "m", "M":
			p.ToggleVolume()
		}
	}

	return p, nil
}

func (p *Player) View() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.err != nil {
		return "Error: " + p.err.Error()
	}
	if p.quitting {
		return ""
	}

	var s string

	if p.currentAudio != nil && p.volume != nil {
		mutedElem := lipgloss.NewStyle().Width(10).MarginRight(1).Render()
		if p.volume.Silent || p.totalVolume == 0 {
			mutedElem = lipgloss.NewStyle().
				Background(styles.ProblemColor).
				Align(lipgloss.Center).
				Width(10).
				MarginRight(1).
				Render(" × Muted ")
		}

		// Percentage of the audio played
		percentage := float64(p.elapsed) / float64(p.duration.Seconds()) * 100

		whiteCell := lipgloss.NewStyle().
			Background(lipgloss.Color("white")).
			Foreground(lipgloss.Color("white")).
			Width(1).
			Height(1).
			Render("█")

		loadBar := lipgloss.NewStyle().
			Align(lipgloss.Left).
			Render(strings.Repeat("•", 100))
		loadBar = strings.Replace(loadBar, "•", whiteCell, int(percentage))
		loadBarBox := lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(100).
			Height(1).
			MaxWidth(100).
			MarginLeft(1).
			Render(loadBar)

		s += lipgloss.JoinHorizontal(lipgloss.Left, mutedElem, loadBarBox)
		s += "\n\n"

		if p.completed {
			s += " ∎ "
		} else {
			if p.running {
				s += " ⏵ "
			} else {
				s += " ⏸ "
			}
		}

		nameElem := styles.PrimaryHighlight(fmt.Sprintf(" ♪ %s ", p.currentAudio.name))

		volumeElem := lipgloss.NewStyle().
			Foreground(styles.PrimaryColor).
			Align(lipgloss.Center).
			Width(15).
			Render(fmt.Sprintf(" λ %d%%", p.totalVolume))

		elapsedStr := FormatSecondsToString(time.Duration(time.Second * p.elapsed))
		elapsedElem := lipgloss.NewStyle().
			Foreground(styles.ContrastColor).
			Render(elapsedStr)

		durationStr := FormatSecondsToString(p.duration)
		durationElem := lipgloss.NewStyle().
			Foreground(styles.ContrastColor).
			Render(durationStr)

		elapseBox := lipgloss.NewStyle().
			Width(28).
			Align(lipgloss.Center).
			Render(fmt.Sprintf(" %s / %s ", elapsedElem, durationElem))

		shortPath := reverseCutString(p.currentAudio.path, PathCharsLimit)
		pathElem := styles.ContrastHighlight(shortPath)

		s += fmt.Sprintf("\t[\t %s • %s • %s • %s \t]",
			nameElem,
			volumeElem,
			elapseBox,
			pathElem,
		)
	}
	s = styles.BaseContainer(s)

	// help
	s += styles.Help("\nℹ: q (quit) | Space (pause/resume) | 🞀 (rewind) | 🞂 (forward) | ⏶ (volume up) | ⏷ (volume down) | m (mute/unmute)\n")

	return s
}

// Reset resets the player state, allowing it to be reused for a new audio file.
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

// Close stops the player and releases resources.
func (p *Player) Close() error {
	var err error
	p.running = false
	if p.stream != nil {
		err = p.stream.Close()
		if err != nil {
			p.err = err
		}
	}
	return err
}

// Quit stops the player and exits the program.
func (p *Player) Quit() tea.Cmd {
	p.quitting = true
	p.Close()
	return tea.Quit
}

// Play starts playing the audio file if it is not already running.
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

func (p *Player) Restart() {
	// Save important state
	auxFile := p.currentAudio
	auxTotalVolume := p.totalVolume
	silent := p.volume.Silent

	p.Reset()

	p.elapsed = 0
	p.duration = p.format.SampleRate.D(p.stream.Len()).Round(time.Second)
	p.currentAudio = auxFile
	p.totalVolume = auxTotalVolume
	p.volume.Silent = silent

	p.stream.Seek(0)
	p.Play()
}

// Resume resumes the audio playback if it is paused.
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

// Stop stops the audio playback if it is currently running.
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

// StopOrResume pauses or resumes the speaker audio depending if it's running or not.
func (p *Player) StopOrResume() {
	if p.running {
		p.Stop()
	} else {
		p.Resume()
	}
}

// DecrementVolume decreases the volume by 5 units, ensuring it does not go below 0.
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

// IncrementVolume increases the volume by 5 units, ensuring it does not exceed 100.
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

// MuteVolume sets the volume to silent, effectively muting the audio.
func (p *Player) MuteVolume() {
	p.volume.Silent = true
}

// UnmuteVolume sets the volume to a non-silent state, allowing audio playback.
func (p *Player) UnmuteVolume() {
	p.volume.Silent = false
}

// ToggleVolume toggles the volume state between muted and unmuted.
func (p *Player) ToggleVolume() {
	if p.volume.Silent {
		p.UnmuteVolume()
	} else {
		p.MuteVolume()
	}
}

func (p *Player) Rewind() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(p.lastSeekTime) < SeekCooldown {
		return
	}

	if p.stream.Err() != nil {
		p.err = p.stream.Err()
		return
	}

	skipDuration := time.Duration(5)
	if p.elapsed+skipDuration >= p.duration {
		skipDuration = 0
	}

	currentPos := p.stream.Position()
	offset := p.format.SampleRate.N(time.Second * skipDuration)

	newPos := currentPos - offset
	newPos = max(newPos, 0)

	p.elapsed = max(p.elapsed-skipDuration, 0)

	if err := p.stream.Seek(newPos); err != nil {
		p.err = err
		return
	}

	p.lastSeekTime = time.Now()
}

func (p *Player) Forward() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(p.lastSeekTime) < SeekCooldown {
		return
	}

	if p.stream.Err() != nil {
		p.err = p.stream.Err()
		return
	}

	skipDuration := time.Duration(5)
	if p.elapsed+skipDuration >= p.duration {
		skipDuration = 0
	}

	currentPos := p.stream.Position()
	offset := p.format.SampleRate.N(time.Second * skipDuration)

	newPos := currentPos + offset
	newPos = min(newPos, p.stream.Len()-1)

	p.elapsed = min(p.elapsed+skipDuration, p.duration)

	if err := p.stream.Seek(newPos); err != nil {
		p.err = err
		return
	}

	p.lastSeekTime = time.Now()
}

// LoadAudio loads the current audio file into the player, decoding it based on its file type.
// Currently it only supports MP3 and WAV formats.
func (p *Player) LoadAudio() {
	if p.currentAudio == nil {
		return
	}

	ext := p.currentAudio.ext
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
	default:
		err = errors.New("invalid file extension")
	}

	if err != nil {
		p.err = err
		return
	}

	// Sample audio
	p.stream = streamer
	p.format = format
	p.duration = format.SampleRate.D(streamer.Len()).Round(time.Second)

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

	if p.totalVolume == 0 {
		p.volume.Silent = true
	}
}

func (p *Player) Error() error {
	return p.err
}

// tick sends a TickMsg every second to update the elapsed time of the audio playback.
func (p *Player) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}

// AbsVolume converts human-readable form volume (0 - 100) to float64 volume.
func AbsVolume(volume int) float64 {
	return (float64(volume) - 100) / 10
}

// FormatSecondsToString formats a total number of seconds into a human-readable string.
// It used for displaying elapsed time in the player.
func FormatSecondsToString(d time.Duration) string {
	totalSeconds := int(d.Seconds())
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

func reverseCutString(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}

	lastNRunes := runes[len(runes)-n:]
	return fmt.Sprintf("...%s", string(lastNRunes))
}
