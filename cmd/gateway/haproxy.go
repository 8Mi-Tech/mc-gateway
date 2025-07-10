package main

import (
	"net"

	proxyproto "github.com/pires/go-proxyproto"
	"github.com/rs/zerolog/log"
)

func haProxyUpstream(source net.Conn, host string) net.Conn {
	target, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		log.Err(err).Msg("failed to resolve TCP address")
		return nil
	}

	conn, err := net.DialTCP("tcp", nil, target)
	if err != nil {
		log.Err(err).Msg("failed to dial TCP")
		return nil
	}

	sourceAddr, err := net.ResolveTCPAddr(
		source.RemoteAddr().Network(),
		source.RemoteAddr().String(),
	)
	if err != nil {
		log.Err(err).Msg("failed to resolve TCP address")
		return nil
	}

	TransportProtocol := proxyproto.TCPv4
	if sourceAddr.IP.To4() == nil {
		TransportProtocol = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           1,
		Command:           proxyproto.PROXY,
		TransportProtocol: TransportProtocol,
		SourceAddr:        sourceAddr,
		DestinationAddr:   target,
	}
	// After the connection was created write the proxy headers first
	_, err = header.WriteTo(conn)
	if err != nil {
		log.Err(err).Msg("failed to write proxy header")
		return nil
	}

	return conn
}
