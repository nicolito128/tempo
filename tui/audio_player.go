package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nicolito128/tempo/sound"
	"github.com/nicolito128/tempo/theme"
)

const (
	PathShowLimit = 32
	WalkMaxDepth  = 10
)

// TickMsg every second of the played audio
type TickMsg struct{}

type AudioPlayer struct {
	state *AudioPlayerState
}

func NewAudioPlayer(opts ...sound.PlayerOpt) *AudioPlayer {
	return &AudioPlayer{
		state: NewAudioPlayerState(opts...),
	}
}

func (ap *AudioPlayer) Model() *AudioPlayerState {
	return ap.state
}

func (ap *AudioPlayer) Append(name string) error {
	return ap.appendAndWalk(name)
}

func (ap *AudioPlayer) appendAndWalk(name string) error {
	info, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("file '%s' not found", name)
		}
		return fmt.Errorf("cannot stat the file '%s': %w", name, err)
	}

	switch mode := info.Mode(); {
	case mode.IsDir():
		err = filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			rel, _ := filepath.Rel(name, path)
			if rel == "." {
				return nil
			}

			depth := 0
			for _, part := range strings.Split(rel, string(os.PathSeparator)) {
				if part != "" {
					depth++
				}
			}

			if depth > WalkMaxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.Type().IsRegular() && hasAudioExt(path) {
				ap.state.queue = append(ap.state.queue, sound.NewPlayer(path))
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot walk the path '%s': %w", name, err)
		}

	case mode.IsRegular() && hasAudioExt(name):
		ap.state.queue = append(ap.state.queue, sound.NewPlayer(name))
	}

	return nil
}

func (ap *AudioPlayer) State() *AudioPlayerState {
	return ap.state
}

type AudioPlayerState struct {
	queue  []*sound.Player
	cursor int

	current *sound.Player

	playOpts []sound.PlayerOpt

	playCh chan *sound.Player

	width  int
	height int
}

func NewAudioPlayerState(opts ...sound.PlayerOpt) *AudioPlayerState {
	return &AudioPlayerState{
		queue:    make([]*sound.Player, 0),
		playCh:   make(chan *sound.Player, 1),
		playOpts: opts,
	}
}

func (s *AudioPlayerState) Queue() []*sound.Player {
	return s.queue
}

func (s *AudioPlayerState) Init() tea.Cmd {
	go func() {
		var active *sound.Player
		var doneCh <-chan struct{}

		for {
			if active != nil {
				doneCh = active.Done()
			}

			select {
			case newSong := <-s.playCh:
				if active != nil && active != newSong {
					active.Pause()
				}
				active = newSong

				switch {
				case active.IsCompleted():
					active.Restart()
				case !active.IsLoaded():
					active.Play(s.playOpts...)
				case active.IsPaused():
					active.Unpause()
				}

			case <-doneCh:
				s.next()
				s.play()
			}
		}
	}()

	return tea.ClearScreen
}

func (s *AudioPlayerState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(s.queue) == 0 {
		return s, tea.Quit
	}
	if s.current == nil {
		s.play()
	}
	if s.current.Err() != nil {
		s.clean()
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, s.tick()

	case TickMsg:
		return s, s.tick()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return s, tea.Quit

		case "+", "up", "k":
			s.current.VolumeInc()

		case "-", "down", "j":
			s.current.VolumeDec()

		case "enter", "space":
			s.togglePause()

		case "left", "h":
			s.current.Rewind()

		case "right", "l":
			s.current.Forward()

		case "m", "M":
			s.toggleVolume()

		case "c", "C":
			s.clean()

		case ",", "[", "<":
			s.prev()
			s.play()

		case ".", "]", ">":
			s.next()
			s.play()
		}
	}

	return s, nil
}

func (s *AudioPlayerState) View() tea.View {
	if len(s.queue) == 0 {
		return tea.NewView("Nothing to play.")
	}
	if s.current == nil {
		return tea.NewView("Tempo is loading...")
	}

	if s.current.Err() != nil {
		return tea.NewView("Tempo error: " + s.current.Err().Error())
	}

	width := s.width
	if width <= 0 {
		width = 80
	}

	var xs string

	mutedElem := lipgloss.NewStyle().Width(10).MarginRight(1).Render()
	if s.current.IsMuted() {
		mutedElem = lipgloss.NewStyle().
			Background(theme.Problem).
			Align(lipgloss.Center).
			Width(10).
			MarginRight(1).
			Render(" × Muted ")
	}

	// Load bar
	percentage := s.current.Elapsed().Seconds() / s.current.Duration().Seconds() * 100

	whiteCell := lipgloss.NewStyle().
		Background(lipgloss.Color("white")).
		Foreground(lipgloss.Color("white")).
		Width(1).
		Height(1).
		Render("█")

	loadBar := lipgloss.NewStyle().
		Align(lipgloss.Left).
		Render(strings.Repeat("•", 100))

	if !math.IsNaN(percentage) {
		loadBar = strings.Replace(loadBar, "•", whiteCell, int(percentage))
	}

	loadBarBox := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(100).
		Height(1).
		MaxWidth(100).
		MarginLeft(1).
		Render(loadBar)

	xs += lipgloss.JoinHorizontal(lipgloss.Left, mutedElem, loadBarBox)
	xs += "\n\n"

	if s.current.IsPaused() {
		xs += " ⏵ "
	} else {
		xs += " ⏸ "
	}

	nameElem := theme.PrimaryHighlight(fmt.Sprintf(" ♪ %s ", s.current.Base()))

	volumeElem := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Align(lipgloss.Center).
		Width(15).
		Render(fmt.Sprintf(" λ %d%%", s.current.Volume()))

	eStr := s.current.Elapsed().String()
	elapsedElem := lipgloss.NewStyle().
		Foreground(theme.Contrast).
		Render(eStr)

	dStr := s.current.Duration().String()
	durationElem := lipgloss.NewStyle().
		Foreground(theme.Contrast).
		Render(dStr)

	elapseBox := lipgloss.NewStyle().
		Width(28).
		Align(lipgloss.Center).
		Render(fmt.Sprintf(" %s / %s ", elapsedElem, durationElem))

	shortPath := reverseCutString(s.current.Path(), PathShowLimit)
	pathElem := theme.ContrastHighlight(shortPath)

	xs += fmt.Sprintf("\t[\t %s • %s • %s • %s \t]",
		nameElem,
		volumeElem,
		elapseBox,
		pathElem,
	)

	// Container box
	xs = theme.BaseContainer(width, 10, xs)
	// Help
	xs += theme.Help("\nℹ: q → quit • ␣ → resume • h → rewind • l → forward • k → volume up • j → volume down • m → mute/unmute • [ → prev • ] → next\n")

	return tea.NewView(xs)
}

func (s *AudioPlayerState) togglePause() {
	if s.current.IsPaused() {
		s.current.Unpause()
	} else {
		s.current.Pause()
	}
}

func (s *AudioPlayerState) toggleVolume() {
	if s.current.IsMuted() {
		s.current.Unmute()
	} else {
		s.current.Mute()
	}
}

func (s *AudioPlayerState) clean() {
	for i := range slices.Backward(s.queue) {
		if s.queue[i].IsCompleted() {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			s.cursor--
		}
	}
}

func (s *AudioPlayerState) next() *sound.Player {
	if len(s.queue) == 0 {
		return nil
	}

	s.cursor = (s.cursor + 1) % len(s.queue)
	return s.queue[s.cursor]
}

func (s *AudioPlayerState) prev() *sound.Player {
	if len(s.queue) == 0 {
		return nil
	}

	s.cursor--
	if s.cursor < 0 {
		s.cursor = len(s.queue) - 1
	}

	return s.queue[s.cursor]
}

func (s *AudioPlayerState) play() {
	if s.cursor < 0 || s.cursor >= len(s.queue) {
		return
	}

	s.current = s.queue[s.cursor]

	if s.current.IsCompleted() || s.current.Elapsed() >= s.current.Duration() {
		_ = s.current.Restart()
	}

	select {
	case s.playCh <- s.current:
	default:
		<-s.playCh
		s.playCh <- s.current
	}
}

func (s *AudioPlayerState) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}
