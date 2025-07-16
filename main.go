package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nicolito128/tempo/internal/components/player"
)

var (
	path = flag.String("p", "", "Load an audio file from the given path")
)

func main() {
	if len(os.Args)-1 <= 0 {
		fmt.Println("Bad arguments: You must set a path to a song")
		os.Exit(1)
	}
	flag.Parse()

	af := player.AudioFile{}
	af.SetName(filepath.Base(*path))
	af.SetPath(*path)

	pm := player.New(75)
	pm.SetAudio(af)

	program := tea.NewProgram(pm)

	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}
