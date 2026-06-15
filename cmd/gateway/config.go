package main

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var (
	configFile = "config.yaml" // 配置文件改为 YAML 格式

	config         Config
	currentLogFile string
	configLoadLock sync.Mutex
)

type (
	Config struct {
		Tcp       TcpConfig                 `yaml:"tcp"`
		Quic      QuicConfig                `yaml:"quic"`
		Kcp       KcpConfig                 `yaml:"kcp"`
		WebSocket WebSocketConfig           `yaml:"websocket"`
		Hosts     map[string]string         `yaml:"hosts"`
		Log       LogConfig                 `yaml:"log"`
		PidFile   string                    `yaml:"pid_file"`
		Plugin    map[string]map[string]any `yaml:"plugin"`
	}

	TcpConfig struct {
		Enable bool `yaml:"enable"`
		Port   int  `yaml:"port"`
	}

	KcpConfig struct {
		Enable       bool `yaml:"enable"`
		Port         int  `yaml:"port"`
		DataShards   int  `yaml:"data_shards"`
		ParityShards int  `yaml:"parity_shards"` // 保持原 TOML 键名，YAML 中键名同样为 parity_Shards
	}

	QuicConfig struct {
		Enable bool     `yaml:"enable"`
		Port   int      `yaml:"port"`
		ALPN   []string `yaml:"alpn"`
	}

	WebSocketConfig struct {
		Enable bool   `yaml:"enable"`
		Port   int    `yaml:"port"`
		Path   string `yaml:"path"`
	}

	// LogConfig 定义了日志的配置，包括日志级别和日志文件路径
	// 日志级别可以是 "trace", "debug", "info", "warn", "error", "fatal", "disabled"
	// 默认日志级别为 "info"
	// 日志文件路径指定了日志输出的位置
	// 如果日志文件路径为空，则日志将只输出到标准输出
	LogConfig struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	}

	// serviceConfig 定义了一个服务的配置，包括是否启用和运行函数
	// 运行函数接收一个 WaitGroup，用于在服务运行时进行同步
	// 这样可以确保所有服务在主函数退出前都能正确关闭
	// 运行函数通常会在 goroutine 中执行，以便并发处理多个服务
	serviceConfig struct {
		enable *bool
		run    func(wg *sync.WaitGroup)
	}
)

var services = []serviceConfig{
	{
		enable: &config.Tcp.Enable,
		run:    runTcp,
	},
	{
		enable: &config.Kcp.Enable,
		run:    runKcp,
	},
	{
		enable: &config.Quic.Enable,
		run:    runQuic,
	},
	{
		enable: &config.WebSocket.Enable,
		run:    runWebSocket,
	},
}

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

	// 使用 yaml.Unmarshal 替代 toml.Unmarshal
	if err := yaml.Unmarshal(byteValue, &config); err != nil {
		return err
	}

	writePIDFile()

	if err := loadLogger(); err != nil {
		return err
	}

	loadPlugins()

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
			case event, ok := <-watcher.Events:
				if !ok {
					log.Error().Msg("watcher.Events channel closed")
					return
				}
				// 检查事件文件名是否匹配 config.yaml
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

func loadPluginConfig(cfg map[string]any, pluginCfg any) error {
	log.Info().
		Any("config", cfg).
		Msg("Loading plugin config")

	// 将 TagName 改为 "yaml" 以匹配结构体标签
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  pluginCfg,
		TagName: "yaml",
	})
	if err != nil {
		return err
	}

	if err := decoder.Decode(cfg); err != nil {
		return err
	}

	return nil
}