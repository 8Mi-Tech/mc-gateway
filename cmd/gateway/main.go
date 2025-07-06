package main

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tursom/mc-gateway/protocol"
)

func main() {
	if err := loadConfig(); err != nil {
		panic(err)
	}

	if err := writePIDFile(); err != nil {
		log.Err(err).Msg("Failed to write PID file")
	}
	defer removePIDFile()

	watcher := watchConfig()
	defer watcher.Close()

	go handleLogRotate()

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, service := range services {
		if !*service.enable {
			continue
		}

		wg.Add(1)
		go service.run(&wg)
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

	client := mapToHost(conn)
	if client == nil {
		return
	}
	defer client.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go copyData(client, conn, &wg)
	copyData(conn, client, nil)

	// 等待所有读写操作完成
	// 不放在 defer 中，以防报错时无法关闭连接
	wg.Wait()
}

func mapToHost(conn net.Conn) net.Conn {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Err(err).
			Str("client", conn.RemoteAddr().String()).
			Msg("failed to reading hostname")
		return nil
	}
	if n == 0 {
		log.Err(errEmptyBuffer).
			Str("client", conn.RemoteAddr().String()).
			Msg("buffer is empty")
		return nil
	}

	mc_host := protocol.GetMcHost(buf[:n])
	if mc_host == "" {
		log.Err(errEmptyBuffer).
			Str("client", conn.RemoteAddr().String()).
			Msg("failed to parse mc host from buffer")
		return nil
	}

	host, ok := config.Hosts[mc_host]
	if !ok {
		host = config.Hosts["default"]
	}
	if host == "" {
		log.Err(errEmptyBuffer).
			Str("client", conn.RemoteAddr().String()).
			Str("host", mc_host).
			Msg("failed to route host")
		return nil
	}

	log.Info().
		Str("client", conn.RemoteAddr().String()).
		Str("host", mc_host).
		Str("mc", host).
		Msg("map to host")

	var client net.Conn

	if host, ok := strings.CutPrefix(host, "quic://"); ok {
		client = upstreamQuic(host)
	} else if host, ok := strings.CutPrefix(host, "kcp://"); ok {
		client = upstreamKcp(host)
	} else {
		client = upstreamTcp(host)
	}
	if client == nil {
		return nil
	}

	client.Write(buf[:n])

	return client
}

func copyData(dst io.Writer, src io.Reader, wg *sync.WaitGroup) {
	defer func() {
		if r := recover(); r != nil {
			var event *zerolog.Event
			if err, ok := r.(error); ok {
				event = log.Err(err)
			} else if str, ok := r.(string); ok {
				event = log.Error().Str("panic", str)
			} else {
				event = log.Error().Any("panic", r)
			}
			event.Msg("Panic in copyData")
		}
	}()

	if wg != nil {
		defer wg.Done()
	}

	_, err := io.Copy(dst, src)
	if err != nil && err != io.EOF {
		log.Err(err).Msg("Error copying data")
	}
}
