package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nicolito128/tempo/internal/components/player"
	"github.com/nicolito128/tempo/internal/components/ui"
)

var (
	play = flag.String("play", "", "Load an audio file from the given path")
	vol  = flag.Int("vol", 50, "Initial volume to play the audio")
)

func main() {
	if len(os.Args)-1 <= 0 {
		fmt.Println("Bad arguments: You must set a path to a song")
		os.Exit(1)
	}
	flag.Parse()

	// Handle error in case the file does not exist
	if _, err := os.Stat(*play); err != nil {
		fmt.Println("Error: the file does not exist")
		os.Exit(1)
	}

	// Handle error in case the file is not a valid audio file (mp3 or wav)
	ext := filepath.Ext(*play)
	if ext != ".mp3" && ext != ".wav" {
		fmt.Println("Error: the file is not a valid audio file. Try using a .mp3 or .wav file")
		os.Exit(1)
	}

	af := player.NewAudioFile(*play)
	tui := ui.New(*vol)
	tui.Player().SetAudioFile(af)

	program := tea.NewProgram(tui)
	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}
