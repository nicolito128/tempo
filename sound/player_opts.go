package sound

type PlayerConfig struct {
	// Start the player in paused mode
	Paused bool
	// Start the player in silent mode
	Silent bool
	// Resample quality
	Quality int
}

func DefaultPlayerConfig() *PlayerConfig {
	cfg := new(PlayerConfig)

	cfg.Paused = false
	cfg.Silent = false

	cfg.Quality = 3

	return cfg
}

type PlayerOpt func(*PlayerConfig)

func WithPlayerPaused(paused bool) PlayerOpt {
	return func(c *PlayerConfig) {
		c.Paused = paused
	}
}

func WithPlayerSilent(silent bool) PlayerOpt {
	return func(c *PlayerConfig) {
		c.Silent = silent
	}
}

func WithPlayerQuality(quality int) PlayerOpt {
	return func(c *PlayerConfig) {
		c.Quality = quality
	}
}
