// pid_unix.go
//go:build unix || plan9

package main

func getPidFileFromConfig() string {
	pidFile := config.PidFile
	if pidFile == "" {
		return "/dev/shm/mc-gateway.pid"
	}
	return pidFile
}
