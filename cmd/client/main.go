package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/rs/zerolog/log"
)

var (
	localPort = 25565

	mcHost = "bh.mc.tursom.cn"
	mcPort = 25565
)

func main() {
	if len(os.Args) > 1 {
		mcHost = os.Args[1]
	}

	// 监听TCP端口
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		log.Fatal().Err(err).
			Int("port", localPort).
			Msg("Failed to listen on port")
	}
	defer listener.Close()
	log.Info().
		Int("port", localPort).
		Msg("Listening for TCP connections")

	for {
		// 接受传入的连接
		conn, err := listener.Accept()
		if err != nil {
			log.Err(err).Msg("Error accepting")
			continue
		}
		log.Info().
			Str("client", conn.RemoteAddr().String()).
			Int("port", localPort).
			Msg("Accepted connection")
		// 处理连接
		go handlerConn(conn)
	}
}

func handlerConn(conn net.Conn) {
	defer conn.Close()

	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // 跳过证书检查
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3s handshake timeout
	defer cancel()

	quicConn, err := quic.DialAddr(ctx, fmt.Sprintf("%s:%d", mcHost, mcPort), tlsConf, nil)
	if err != nil {
		log.Err(err).Msg("Failed to dial QUIC")
		return
	}

	stream, err := quicConn.OpenStream()
	if err != nil {
		log.Err(err).Msg("Failed to open stream")
		return
	}
	defer stream.Close()
	log.Info().
		Msg("QUIC stream opened")

	// read and write stream data

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Err(err).
			Str("client", conn.RemoteAddr().String()).
			Msg("Error reading hostname")
		return
	}
	if n == 0 {
		log.Err(errors.New("empty buffer")).
			Str("client", conn.RemoteAddr().String()).
			Msg("Error: buffer is empty")
		return
	}
	buf = buf[:n]

	new_buf := replaceMcHost(buf, mcHost)
	_, err = stream.Write(new_buf) // 写入数据到 QUIC 流
	if err != nil {
		log.Err(err).
			Str("client", conn.RemoteAddr().String()).
			Str("host", mcHost).
			Int("port", mcPort).
			Msg("Error writing to QUIC stream")
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go copyData(stream, conn, &wg)
	copyData(conn, stream, nil)
	wg.Wait()
}

func copyData(src io.Reader, dst io.Writer, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	_, err := io.Copy(dst, src)
	if err != nil && err != io.EOF {
		log.Err(err).Msg("Error copying data")
	}
}

func replaceMcHost(buf []byte, host string) []byte {
	if len(buf) < 5 {
		return nil
	}

	var out bytes.Buffer
	head := buf[:4]

	buf = buf[4:]
	host_len := buf[0]
	if len(buf) < int(host_len)+1 {
		return nil
	}

	raw_host := string(buf[1 : host_len+1])
	if spliterIndex := strings.IndexRune(raw_host, 0); spliterIndex != -1 {
		host = host + raw_host[spliterIndex:]
	}

	// 修改标识数据包长度的字节
	head[0] += byte(len(host) - len(raw_host))

	out.Write(head)                // 保留前四个字节
	out.WriteByte(byte(len(host))) // 写入主机名长度
	out.Write([]byte(host))        // 写入主机名

	out.Write(buf[host_len+1:]) // 写入剩余数据

	return out.Bytes()
}
