package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	Config struct {
		Port    int               `json:"port"`
		Hosts   map[string]string `json:"hosts"`
		Default string            `json:"default"`
		Log     LogConfig         `json:"log"`
	}

	LogConfig struct {
		Level string `json:"level"`
		File  string `json:"file"`
	}
)

var (
	config         Config
	currentLogFile string
	configLoadLock sync.Mutex
)

func loadConfig() error {
	configLoadLock.Lock()
	defer configLoadLock.Unlock()

	file, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(byteValue, &config); err != nil {
		return err
	}

	return loadLogger()
}

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

func watchConfig() *fsnotify.Watcher {
	go func() {
		for {
			time.Sleep(time.Minute)
			if err := loadConfig(); err != nil {
				log.Error().Err(err).Msg("Failed to reload config")
			}
		}
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create config watcher")
	}
	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					log.Error().Msg("watcher.Events channel closed")
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Error().Msg("watcher.Errors channel closed")
					return
				}
				log.Error().Err(err).Msg("watcher error")
				continue
			}

			log.Info().Msg("reload config")
			if err := loadConfig(); err != nil {
				log.Error().Err(err).Msg("Failed to reload config")
			}
		}
	}()
	if err = watcher.Add("."); err != nil {
		log.Fatal().Err(err).Msg("Failed to watch config file")
	}

	return watcher
}

func main() {
	if err := loadConfig(); err != nil {
		panic(err)
	}

	watcher := watchConfig()
	defer watcher.Close()

	// 监听TCP端口
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Info().
		Int("port", config.Port).
		Msg("Listening")

	for {
		// 接受传入的连接
		conn, err := listener.Accept()
		if err != nil {
			log.Err(err).Msg("Error accepting")
			continue
		}
		// 处理连接
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}

		if err, ok := rec.(error); ok {
			log.Err(err).
				Str("client", conn.RemoteAddr().String()).
				Msg("Panic on handle request")
		} else {
			log.Error().Any("err", rec).
				Str("client", conn.RemoteAddr().String()).
				Msg("Panic on handle request")
		}
	}()

	// 确保连接关闭
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Err(err).
			Str("client", conn.RemoteAddr().String()).
			Msg("Error reading hostname")
		return
	}
	if n == 0 {
		log.Err(errEmptyBuffer).
			Str("client", conn.RemoteAddr().String()).
			Msg("Error: buffer is empty")
		return
	}
	mc_host := getMcHost(buf[:n])
	host, ok := config.Hosts[mc_host]
	if !ok {
		host = config.Default
	}

	log.Info().
		Str("client", conn.RemoteAddr().String()).
		Str("mc", host).
		Msg("map to host")

	client, err := net.Dial("tcp", host)
	if err != nil {
		log.Err(err).Msg("Error dialing")
		return
	}
	defer client.Close()

	client.Write(buf[:n])

	var wg sync.WaitGroup
	wg.Add(2)

	go handleRead(client, conn, &wg)
	go handleWrite(client, conn, &wg)

	wg.Wait()
}

func handleRead(srv, cli net.Conn, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		srv.Close()
		cli.Close()
	}()

	buf := make([]byte, 1024)

	for {
		n, err := srv.Read(buf)
		if err != nil {
			log.Err(err).Msg("Error reading from server")
			return
		}

		if _, err := cli.Write(buf[:n]); err != nil {
			log.Err(err).Msg("Error writing to client")
			return
		}
	}
}

func handleWrite(srv, cli net.Conn, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		srv.Close()
		cli.Close()
	}()

	buf := make([]byte, 1024)

	for {
		n, err := cli.Read(buf)
		if err != nil {
			log.Err(err).Msg("Error reading from client")
			return
		}

		if _, err := srv.Write(buf[:n]); err != nil {
			log.Err(err).Msg("Error writing to server")
			return
		}
	}
}

func getMcHost(buf []byte) string {
	if len(buf) < 5 {
		return ""
	}

	buf = buf[4:]
	host_len := buf[0]
	if len(buf) < int(host_len)+1 {
		return ""
	}

	host := string(buf[1 : host_len+1])

	if spliterIndex := strings.IndexRune(host, 0); spliterIndex != -1 {
		return host[0:spliterIndex]
	} else {
		return host
	}
}
