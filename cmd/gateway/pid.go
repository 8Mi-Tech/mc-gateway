package main

import (
	"fmt"
	"os"
)

var currentPidFile string

func writePIDFile() error {
	newPidFile := getPidFileFromConfig()
	if newPidFile == currentPidFile {
		return nil
	}

	if currentPidFile != "" {
		removePIDFile()
	}

	pid := os.Getpid()
	if err := os.WriteFile(newPidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		return err
	}

	currentPidFile = newPidFile
	return nil
}

func removePIDFile() {
	os.Remove(currentPidFile)
}
