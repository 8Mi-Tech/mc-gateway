package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

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

	// 启动QUIC服务
	if config.Quic.Enable {
		wg.Add(1)
		go runQuic(&wg)
	}

	if config.Kcp.Enable {
		wg.Add(1)
		go runKcp(&wg)
	}

	// 监听TCP端口
	if config.Tcp.Enable {
		wg.Add(1)
		go runTcp(&wg)
	}
}

func runTcp(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Tcp.Port))
	if err != nil {
		log.Fatal().Err(err).
			Int("port", config.Tcp.Port).
			Msg("Failed to listen on port")
	}
	defer listener.Close()
	log.Info().
		Int("port", config.Tcp.Port).
		Msg("Listening for TCP connections")

	for {
		// 接受传入的连接
		conn, err := listener.Accept()
		if err != nil {
			log.Err(err).Msg("Error accepting")
			continue
		}
		setSocketOptions(conn)
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

	client := mapToHost(conn)
	if client == nil {
		return
	}
	defer client.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	go handleRead(client, conn, &wg)
	handleWrite(client, conn, nil)

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

func upstreamTcp(host string) net.Conn {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Err(err).Str("host", host).Msg("Error dialing upstream")
		return nil
	}
	setSocketOptions(conn)
	return conn

}

func handleRead(srv, cli net.Conn, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	_, err := io.Copy(srv, cli)
	if err != nil && err != io.EOF {
		log.Err(err).Msg("Error copying data")
	}
}

func handleWrite(srv, cli net.Conn, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	_, err := io.Copy(cli, srv)
	if err != nil && err != io.EOF {
		log.Err(err).Msg("Error copying data")
	}
}

func setSocketOptions(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true) // 禁用 Nagle 算法
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}
}
