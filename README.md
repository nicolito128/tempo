# Tempo

A simple TUI music player written in Go.

![Screenshot of Tempo in Kitty](./resources/screenshot1-kitty.png)

## Requirements

- Go 1.25+
- alsa-lib

## Getting started

Compile the program:

    go build -o bin/tempo cmd/tempo/main.go

Then

    bin/tempo -play <path_to_song>.mp3
