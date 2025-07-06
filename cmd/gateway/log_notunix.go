// pid_unix.go
//go:build !unix && !plan9

package main

import (
	"github.com/rs/zerolog/log"
)

func handleLogRotate() {
	// No-op for non-unix platforms
	// Log rotation is not supported on this platform
	// This function can be left empty or removed if not needed
	log.Info().Msg("Log rotation is not supported on this platform")
}
