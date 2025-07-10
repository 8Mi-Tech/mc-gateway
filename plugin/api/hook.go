package api

import (
	"errors"
	"net"
	"unsafe"
)

var (
	UnsupportedHookType = errors.New("unsupported hook type")
)

type (
	HookType[Accept, Handler any] struct {
		key string
	}

	HookHandler[Accept, Handler any] struct {
		acceptor Accept
		handler  Handler
	}
)

var (
	HookUpstream = HookType[
		func(source net.Conn, host string) bool,
		func(source net.Conn, host string) (net.Conn, error),
	]{
		key: "upstream",
	}
)

func (h HookType[Accept, Handler]) Key() string {
	return h.key
}

func (h HookType[Accept, Handler]) AsAny() HookType[any, any] {
	return *(*HookType[any, any])(unsafe.Pointer(&h))
}

func (h HookHandler[Acceptor, Handler]) Acceptor() Acceptor {
	return h.acceptor
}

func (h HookHandler[Acceptor, Handler]) Handler() Handler {
	return h.handler
}
