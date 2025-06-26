package main

import (
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func loadLogger() error {
	level, err := zerolog.ParseLevel(config.Log.Level)
	if err != nil {
		return err
	}

	log.Logger = log.Logger.Level(level)

	if config.Log.File != "" && currentLogFile != config.Log.File {
		if err := os.MkdirAll(filepath.Dir(config.Log.File), 0755); err != nil {
			return err
		}

		logFile, err := os.OpenFile(config.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		log.Logger = log.Logger.Output(io.MultiWriter(os.Stdout, logFile))

		currentLogFile = config.Log.File
	}

	return nil
}

func handleLogRotate() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP)

	for range signalChan {
		log.Info().Msg("Received SIGHUP, reopening log file")
		if err := reopenLogFile(); err != nil {
			log.Error().Err(err).Msg("Failed to reopen log file")
		}
	}
}

func reopenLogFile() error {
	configLoadLock.Lock()
	defer configLoadLock.Unlock()

	if config.Log.File == "" {
		return nil
	}

	// 重新打开日志文件
	logFile, err := os.OpenFile(config.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	log.Logger = log.Logger.Output(io.MultiWriter(os.Stdout, logFile))
	currentLogFile = config.Log.File

	return nil
}
