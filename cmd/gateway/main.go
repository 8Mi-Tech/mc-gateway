package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port    int               `json:"port"`
	Hosts   map[string]string `json:"hosts"`
	Default string            `json:"default"`
}

var config Config

func loadConfig() error {
	file, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(byteValue, &config)
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
			}

			log.Info().Msg("reload config")
			if err := loadConfig(); err != nil {
				log.Error().Err(err).Msg("Failed to reload config")
			}
		}
	}()
	err = watcher.Add(".")
	if err != nil {
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
			log.Err(err).Msg("Panic on handle request")
		} else {
			log.Error().Any("err", rec).Msg("Panic on handle request")
		}
	}()

	// 确保连接关闭
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Err(err).Msg("Error reading hostname")
		return
	}
	mc_host := GetMcHost(buf[:n])
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
			return
		}

		cli.Write(buf[:n])
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
			return
		}

		srv.Write(buf[:n])
	}
}

func GetMcHost(buf []byte) string {
	if len(buf) < 5 {
		return ""
	}

	buf = buf[4:]
	host_len := buf[0]
	if len(buf)+1 < int(host_len) {
		return ""
	}

	host := string(buf[1 : host_len+1])
	
	if spliterIndex := strings.IndexRune(host, 0); spliterIndex != -1 {
		return host[0:spliterIndex]
	} else {
		return host
	}    
}
