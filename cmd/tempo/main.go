package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/nicolito128/tempo/sound"
	"github.com/nicolito128/tempo/tui"

	"github.com/spf13/cobra"
)

var (
	pauseFlag   bool
	muteFlag    bool
	shuffleFlag bool
	volumeFlag  int
)

var rootCmd = &cobra.Command{
	Use:   "tempo",
	Short: "tempo is a terminal music player",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		opts := make([]sound.PlayerOpt, 0)
		opts = append(opts,
			sound.WithPlayerPaused(pauseFlag),
			sound.WithPlayerSilent(muteFlag),
			sound.WithPlayerVolume(volumeFlag),
		)

		ap := tui.NewAudioPlayer(opts...)
		for _, arg := range args {
			ap.Append(arg)
		}

		queue := ap.Model().Queue()
		if len(queue) == 0 {
			fmt.Println("Tempo has nothing to play. Try suplying files with the following extensions: .mp3, .wav, .ogg, .flac.")
			return nil
		}
		if shuffleFlag {
			rand.Shuffle(len(queue), func(i, j int) {
				queue[i], queue[j] = queue[j], queue[i]
			})
		}

		if err := sound.InitAudioSystem(); err != nil {
			return err
		}
		clearConsole()

		prog := tea.NewProgram(ap.Model())
		if _, err := prog.Run(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&pauseFlag, "pause", "p", false, "Start the sound player in paused mode")
	rootCmd.Flags().BoolVarP(&muteFlag, "mute", "m", false, "Start the sound player in mute mode")
	rootCmd.Flags().BoolVar(&shuffleFlag, "shuffle", false, "Shuffle the queue")

	rootCmd.Flags().IntVarP(&volumeFlag, "volume", "v", sound.DefaultInitVolume, "Start the sound player with the given volume percent")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Tempo, there's been an error: %v\n", err)
	}
}

func clearConsole() {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}
