package main

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

var (
	configFile = "config.toml"

	config         Config
	currentLogFile string
	configLoadLock sync.Mutex
)

type (
	Config struct {
		Tcp     ProtocolConfig    `toml:"tcp"`
		Quic    QuicConfig        `toml:"quic"`
		Kcp     KcpConfig         `toml:"kcp"`
		Hosts   map[string]string `toml:"hosts"`
		Log     LogConfig         `toml:"log"`
	}

	ProtocolConfig struct {
		Enable bool `toml:"enable"`
		Port   int  `toml:"port"`
	}

	KcpConfig struct {
		Enable       bool `toml:"enable"`
		Port         int  `toml:"port"`
		DataShards   int  `toml:"data_shards"`
		ParityShards int  `toml:"parity_Shards"`
	}

	QuicConfig struct {
		Enable             bool     `toml:"enable"`
		Port               int      `toml:"port"`
		ApplicionProtocols []string `toml:"application_protocols"`
	}

	LogConfig struct {
		Level string `toml:"level"`
		File  string `toml:"file"`
	}
)

func loadConfig() error {
	configLoadLock.Lock()
	defer configLoadLock.Unlock()

	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := toml.Unmarshal(byteValue, &config); err != nil {
		return err
	}

	return loadLogger()
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
			case event, ok := <-watcher.Events:
				if !ok {
					log.Error().Msg("watcher.Events channel closed")
					return
				}
				if !strings.HasSuffix(event.Name, configFile) {
					continue
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
