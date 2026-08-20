package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/nicolito128/tempo/sound"

	"github.com/spf13/cobra"
)

var (
	pausedFlag bool
	silentFlag bool

	volumeFlag int
)

var rootCmd = &cobra.Command{
	Use:   "tempo",
	Short: "tempo is a terminal music player",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if err := sound.InitAudioSystem(); err != nil {
			panic(err)
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		queue := make([]*sound.Player, len(args))
		for i := range len(args) {
			queue[i] = sound.NewPlayer(args[i])
		}

		clearConsole()

	outer:
		for _, q := range queue {
			defer q.Close()

			q.SetVolume(volumeFlag)
			if err := q.Play(sound.WithPlayerPaused(pausedFlag), sound.WithPlayerSilent(silentFlag)); err != nil {
				fmt.Println(err)
			}

			fmt.Println(q.Path())

		inner:
			for {
				select {
				case <-sigs:
					break outer
				case <-q.Done():
					break inner
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&pausedFlag, "paused", "p", false, "Start the sound player in paused mode")
	rootCmd.Flags().BoolVarP(&silentFlag, "silent", "s", false, "Start the sound player in silent mode")

	rootCmd.Flags().IntVarP(&volumeFlag, "volume", "v", sound.DefaultInitVolume, "Start the sound player with the given percentual volume")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
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
