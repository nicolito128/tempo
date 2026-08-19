package sound

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

const (
	DefaultInitVolume         int     = 100
	DefaultPercentVolumeShift int     = 5
	DefaultVolumeShift        float64 = 0.10
)

const (
	SeekCooldown     time.Duration   = 10 * time.Millisecond
	GlobalSampleRate beep.SampleRate = beep.SampleRate(48000)
)

func InitAudioSystem() error {
	return speaker.Init(GlobalSampleRate, GlobalSampleRate.N(time.Second/10))
}

func Play(path string, paused, silent bool) error {
	return NewPlayer(path).Play(paused, silent)
}

type Player struct {
	path, ext, base string

	file *os.File

	streamer beep.StreamSeekCloser
	format   beep.Format

	pauseCtl  *beep.Ctrl
	volumeCtl *effects.Volume

	percentVolume int

	duration time.Duration
	elapsed  time.Duration

	lastSeek time.Time

	quitch chan struct{}

	mu sync.RWMutex
}

func NewPlayer(filename string) *Player {
	p := new(Player)

	p.path = path.Clean(filename)
	p.ext = path.Ext(filename)
	p.base = path.Base(filename)

	p.percentVolume = DefaultInitVolume

	p.quitch = make(chan struct{}, 1)

	return p
}

func (p *Player) Path() string {
	return p.path
}

func (p *Player) Ext() string {
	return p.ext
}

func (p *Player) Base() string {
	return p.base
}

func (p *Player) Volume() int {
	return p.percentVolume
}

func (p *Player) SetVolume(value int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.percentVolume = max(0, min(value, 120))
	p.updateVolume()
}

func (p *Player) Duration() time.Duration {
	return p.duration
}

func (p *Player) Done() chan struct{} {
	return p.quitch
}

func (p *Player) Play(paused, silent bool) error {
	if p.path == "." {
		return errors.New("empty filepath")
	}

	var err error

	f, err := os.OpenFile(p.path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("player open file error: %w", err)
	}
	p.file = f

	var (
		streamer beep.StreamSeekCloser
		format   beep.Format
	)

	switch p.ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(p.file)
	case ".wav":
		streamer, format, err = wav.Decode(p.file)
	case ".flac":
		streamer, format, err = flac.Decode(p.file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(p.file)
	default:
		err = errors.New("not supported file format")
	}

	if err != nil {
		return err
	}
	p.streamer, p.format = streamer, format

	var resampledStreamer beep.Streamer = p.streamer
	if format.SampleRate != GlobalSampleRate {
		resampledStreamer = beep.Resample(3, format.SampleRate, GlobalSampleRate, p.streamer)
	}

	p.pauseCtl = &beep.Ctrl{
		Streamer: resampledStreamer,
		Paused:   paused,
	}

	p.volumeCtl = &effects.Volume{
		Streamer: p.pauseCtl,
		Base:     2.0,
		Volume:   0,
		Silent:   silent,
	}
	p.updateVolume()

	p.duration = format.SampleRate.D(streamer.Len()).Round(time.Second)

	speaker.Play(p.volumeCtl)

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			p.mu.Lock()

			if p.streamer != nil {
				p.elapsed = p.format.SampleRate.D(p.streamer.Position()).Round(time.Second)
			}

			if p.elapsed >= p.duration {
				p.mu.Unlock()
				select {
				case p.quitch <- struct{}{}:
				default:
				}
				return
			}
			p.mu.Unlock()
		}
	}()

	return nil
}

func (p *Player) Close() error {
	if p.file != nil {
		if p.streamer == nil {
			return errors.New("invalid streamer")
		}
		if err := p.streamer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Player) Pause() {
	if p.streamer == nil {
		return
	}
	if p.pauseCtl == nil {
		return
	}
	p.pauseCtl.Paused = true
}

func (p *Player) Unpause() {
	if p.streamer == nil {
		return
	}
	if p.pauseCtl == nil {
		return
	}
	p.pauseCtl.Paused = false
}

func (p *Player) VolumeInc() {
	if p.streamer == nil {
		return
	}
	if p.volumeCtl == nil {
		return
	}
	p.SetVolume(p.percentVolume + DefaultPercentVolumeShift)
}

func (p *Player) VolumeDec() {
	if p.streamer == nil {
		return
	}
	if p.volumeCtl == nil {
		return
	}
	p.SetVolume(p.percentVolume - DefaultPercentVolumeShift)
}

func (p *Player) Mute() {
	if p.streamer == nil {
		return
	}
	if p.volumeCtl == nil {
		return
	}
	p.volumeCtl.Silent = true
}

func (p *Player) Unmute() {
	if p.streamer == nil {
		return
	}
	if p.volumeCtl == nil {
		return
	}
	p.volumeCtl.Silent = false
}

func (p *Player) Restart() error {
	if p.streamer == nil {
		return errors.New("invalid streamer")
	}
	if p.volumeCtl == nil {
		return errors.New("invalid volume controller")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.elapsed = 0
	p.volumeCtl.Silent = false

	return p.streamer.Seek(0)
}

func (p *Player) Rewind() error {
	if p.streamer == nil {
		return errors.New("invalid streamer")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(p.lastSeek) < SeekCooldown {
		return errors.New("before seek cooldown")
	}

	step := 5 * time.Second

	currentPos := p.streamer.Position()
	offset := p.format.SampleRate.N(step)

	newPos := max(currentPos-offset, 0)
	p.elapsed = p.format.SampleRate.D(newPos).Round(time.Second)

	if err := p.streamer.Seek(newPos); err != nil {
		return err
	}

	p.lastSeek = time.Now()
	return nil
}

func (p *Player) Forward() error {
	if p.streamer == nil {
		return errors.New("invalid streamer")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(p.lastSeek) < SeekCooldown {
		return errors.New("before seek cooldown")
	}

	step := 5 * time.Second

	currentPos := p.streamer.Position()
	offset := p.format.SampleRate.N(step)

	newPos := min(currentPos+offset, p.streamer.Len()-1)
	p.elapsed = p.format.SampleRate.D(newPos).Round(time.Second)

	if err := p.streamer.Seek(newPos); err != nil {
		return err
	}

	p.lastSeek = time.Now()
	return nil
}

func (p *Player) updateVolume() {
	if p.volumeCtl == nil {
		return
	}

	if p.percentVolume <= 0 {
		p.percentVolume = 0
		p.volumeCtl.Silent = true
		return
	}

	p.volumeCtl.Silent = false

	factor := float64(p.percentVolume) / 100.0
	p.volumeCtl.Volume = math.Log2(factor)
}
