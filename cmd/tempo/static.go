//go:build static

package main

// #cgo CFLAGS: -I../../libs/alsa-lib-1.2.14/src/include/
// #cgo LDFLAGS: -L../../libs/alsa-lib-1.2.14/src/.libs/ -lasound -lm
import "C"
