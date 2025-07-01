package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/xtaci/kcp-go"
)

func runKcp(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	listener, err := kcp.ListenWithOptions(fmt.Sprintf(":%d", config.Kcp.Port), nil, config.Kcp.DataShards, config.Kcp.ParityShards)
	if err != nil {
		log.Fatal().Err(err).
			Int("port", config.Kcp.Port).
			Msg("Failed to listen on KCP port")
	}
	defer listener.Close()

	log.Info().Int("port", config.Kcp.Port).Msg("KCP server is listening")

	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			log.Err(err).
				Msg("Failed to accept KCP connection")
			continue
		}
		log.Info().
			Str("remote_addr", conn.RemoteAddr().String()).
			Msg("Accepted KCP connection")

		conn.SetACKNoDelay(true)

		go handleRequest(conn)
	}
}

func upstreamKcp(host string) net.Conn {
	conn, err := kcp.DialWithOptions(host, nil, config.Kcp.DataShards, config.Kcp.ParityShards)
	if err != nil {
		log.Error().Err(err).
			Msg("Failed to dial KCP server")
	}
	defer conn.Close()

	conn.SetACKNoDelay(true)
	return conn
}
