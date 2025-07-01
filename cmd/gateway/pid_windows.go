// pid_windows.go
//go:build windows

package main

func getPidFileFromConfig() string {
	return config.PidFile
}
