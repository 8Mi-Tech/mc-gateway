package protocol

import (
	"bytes"
	"strings"
)

// ReplaceMcHost 替换 Minecraft 主机名
// 必须是连接的第一个数据包
func ReplaceMcHost(buf []byte, host string) []byte {
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

// GetMcHost 通过第一个数据包获取 Minecraft 主机名
func GetMcHost(buf []byte) string {
	if len(buf) < 5 {
		return ""
	}

	buf = buf[4:]
	host_len := buf[0]
	if len(buf) < int(host_len)+1 {
		return ""
	}

	host := string(buf[1 : host_len+1])

	if spliterIndex := strings.IndexRune(host, 0); spliterIndex != -1 {
		return host[0:spliterIndex]
	} else {
		return host
	}
}
