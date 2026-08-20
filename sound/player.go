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
	MaxInMemorySize  int64           = 1 * 1024 * 1024 // 1 MiB
)

func InitAudioSystem() error {
	return speaker.Init(GlobalSampleRate, GlobalSampleRate.N(time.Second/10))
}

type Player struct {
	conf *PlayerConfig

	path, ext, base string

	info os.FileInfo
	file *os.File

	streamer beep.Streamer
	format   beep.Format

	pauseCtl  *beep.Ctrl
	volumeCtl *effects.Volume

	percentVolume int

	duration time.Duration
	elapsed  time.Duration

	lastSeek time.Time

	donech chan struct{}

	completed bool
	closed    bool

	mu sync.RWMutex
}

func NewPlayer(filename string, opts ...PlayerOpt) *Player {
	p := new(Player)

	cfg := DefaultPlayerConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	p.conf = cfg

	p.path = path.Clean(filename)
	p.ext = path.Ext(filename)
	p.base = path.Base(filename)

	p.percentVolume = DefaultInitVolume

	p.donech = make(chan struct{}, 1)

	return p
}

func (p *Player) Config() *PlayerConfig {
	return p.conf
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

func (p *Player) IsMuted() bool {
	if p.streamer == nil {
		return false
	}
	if p.volumeCtl != nil {
		return p.volumeCtl.Silent
	}
	return p.percentVolume == 0
}

func (p *Player) IsPaused() bool {
	if p.pauseCtl != nil {
		return p.pauseCtl.Paused
	}
	return false
}

func (p *Player) IsCompleted() bool {
	return p.completed
}

func (p *Player) IsLoaded() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.streamer != nil
}

func (p *Player) Elapsed() time.Duration {
	return p.elapsed
}

func (p *Player) Duration() time.Duration {
	return p.duration
}

func (p *Player) Done() chan struct{} {
	return p.donech
}

func (p *Player) Play(opts ...PlayerOpt) error {
	if p.closed {
		return errors.New("file streaming already closed")
	}

	for _, opt := range opts {
		opt(p.conf)
	}

	if p.path == "." {
		return errors.New("invalid file to play")
	}

	var err error

	info, err := os.Stat(p.path)
	if err != nil {
		return fmt.Errorf("player stat file error: %w", err)
	}
	p.info = info

	if p.info.IsDir() {
		return errors.New("cannot use a directory as a file to play")
	}

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

	// TODO: maybe add support to .midi files

	default:
		err = errors.New("not supported file format")
	}

	if err != nil {
		return err
	}
	p.streamer, p.format = streamer, format

	// for some tiny files just fill the buffer with the whole audio file (1 MiB)
	var buffer *beep.Buffer
	if p.info.Size() < MaxInMemorySize {
		buffer = beep.NewBuffer(format)
		buffer.Append(streamer)

		streamer.Close()
		if p.file != nil {
			p.file.Close()
		}
	}

	if buffer != nil {
		p.streamer = buffer.Streamer(0, buffer.Len())
	}

	// resample the audio to avoid weird bitrate
	// maybe the global sample rate is a bad idea, TODO: investigate
	resampledStreamer := p.streamer
	if format.SampleRate != GlobalSampleRate {
		resampledStreamer = beep.Resample(p.conf.Quality, format.SampleRate, GlobalSampleRate, p.streamer)
	}

	p.pauseCtl = &beep.Ctrl{
		Streamer: resampledStreamer,
		Paused:   p.conf.Paused,
	}
	// by the docs, it is necessary to pass the Ctrl to the volume ctl
	p.volumeCtl = &effects.Volume{
		Streamer: p.pauseCtl,
		Base:     2.0,
		Volume:   0,
		Silent:   p.conf.Silent,
	}
	p.updateVolume()

	// get the duration
	p.duration = format.SampleRate.D(streamer.Len()).Round(time.Second)

	// init volume percent
	p.SetVolume(p.conf.Volume)

	p.startPlayback()
	return nil
}

func (p *Player) Err() error {
	if p.streamer != nil {
		return p.streamer.Err()
	}
	return nil
}

func (p *Player) Close() error {
	if p.file != nil {
		if p.streamer == nil {
			return errors.New("invalid streamer")
		}

		if v, ok := p.streamer.(beep.StreamCloser); ok {
			if err := v.Close(); err != nil {
				return err
			}
		}
	}
	p.closed = true
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

	seeker, ok := p.streamer.(beep.StreamSeeker)
	if !ok {
		return errors.New("cannot convert beep.Streamer to beep.StreamSeeker")
	}

	p.mu.Lock()

	wasCompleted := p.completed

	p.elapsed = 0
	p.volumeCtl.Silent = false

	if p.pauseCtl != nil {
		p.pauseCtl.Paused = false
	}
	p.mu.Unlock()

	speaker.Lock()
	err := seeker.Seek(0)
	speaker.Unlock()

	if wasCompleted {
		p.startPlayback()
	}

	return err
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

	seeker, ok := p.streamer.(beep.StreamSeeker)
	if !ok {
		return errors.New("cannot convert beep.Streamer to beep.StreamSeeker")
	}

	step := 5 * time.Second

	currentPos := seeker.Position()
	offset := p.format.SampleRate.N(step)

	newPos := max(currentPos-offset, 0)
	p.elapsed = p.format.SampleRate.D(newPos).Round(time.Second)

	if err := seeker.Seek(newPos); err != nil {
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

	seeker, ok := p.streamer.(beep.StreamSeeker)
	if !ok {
		return errors.New("cannot convert beep.Streamer to beep.StreamSeeker")
	}

	step := 5 * time.Second

	currentPos := seeker.Position()
	offset := p.format.SampleRate.N(step)

	maxPos := max(0, seeker.Len()-500)
	newPos := min(currentPos+offset, maxPos)

	speaker.Lock()
	err := seeker.Seek(newPos)
	speaker.Unlock()

	if err != nil {
		return err
	}

	p.elapsed = p.format.SampleRate.D(newPos).Round(time.Second)
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

func (p *Player) startPlayback() {
	p.mu.Lock()
	p.completed = false
	p.mu.Unlock()

	speaker.Play(
		beep.Seq(p.volumeCtl, beep.Callback(func() {
			p.mu.Lock()
			p.completed = true
			p.mu.Unlock()

			select {
			case p.donech <- struct{}{}:
			default:
			}
		})),
	)

	go func() {
		step := 250 * time.Millisecond
		ticker := time.NewTicker(step)
		defer ticker.Stop()

		for range ticker.C {
			p.mu.Lock()

			if p.closed || p.completed {
				p.mu.Unlock()
				return
			}

			if p.streamer != nil {
				if v, ok := p.streamer.(beep.StreamSeeker); ok {
					p.elapsed = p.format.SampleRate.D(v.Position()).Round(time.Second)
				} else {
					p.elapsed += step
				}
			}

			p.mu.Unlock()
		}
	}()
}
