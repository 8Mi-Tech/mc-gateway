package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源的连接（生产环境中应该更严格）
		return true
	},
}

type (
	// WebSocket 连接适配器，实现 net.Conn 接口
	webSocketConn struct {
		*websocket.Conn
		messageRemain []byte // 用于存储未处理的消息
	}
)

// WebSocket 处理函数
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	handleRequest(&webSocketConn{Conn: conn})
}

func (w *webSocketConn) Read(b []byte) (n int, err error) {
	// 如果有未处理的消息，直接从 messageRemain 中读取
	if len(w.messageRemain) > 0 {
		copied := copy(b, w.messageRemain)
		if copied < len(w.messageRemain) {
			w.messageRemain = w.messageRemain[copied:]
		} else {
			w.messageRemain = nil // 清空已处理的消息
		}
		return copied, nil
	}

	// 读取新消息
	_, message, err := w.ReadMessage()
	if err != nil {
		return 0, err
	}
	copied := copy(b, message)
	if copied < len(message) {
		w.messageRemain = message[copied:]
	}
	return copied, nil
}

func (w *webSocketConn) Write(b []byte) (n int, err error) {
	if err = w.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *webSocketConn) SetDeadline(t time.Time) error {
	return w.SetReadDeadline(t)
}

// 启动 WebSocket 服务器
func runWebSocket(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	path := config.WebSocket.Path
	if path == "" {
		path = "/" // 默认路径，全部处理
	}
	http.HandleFunc(path, handleWebSocket)

	port := config.WebSocket.Port
	if port == 0 {
		port = 8080 // 默认端口
	}

	log.Info().Int("port", port).Str("path", path).Msg("Starting WebSocket server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal().Err(err).Msg("Failed to start WebSocket server")
	}
}
