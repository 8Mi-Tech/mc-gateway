package main

import (
	"fmt"
	"os"
)

func writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile("/dev/shm/mc-gateway.pid", []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

func removePIDFile() {
	os.Remove("/dev/shm/mc-gateway.pid")
}
