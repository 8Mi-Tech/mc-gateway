
# mc-gateway

一个简易的 Minecraft 网关，通过 host 将客户端的流量转发到对应的后端 Minecraft 服务器。

## 配置

目前 mc-gateway 只支持读取当前目录的 `config.yaml` 作为配置。<br>
在 `config.yaml` 被修改时，可以自动加载并更新部分配置，以达到不停机修改配置的效果。

**支持热加载的配置有：**
- `hosts`
- `log`
- KCP 的 `data_shards` 和 `parity_shards`
- QUIC 的 `alpn`
- `pid_file`

### 详细配置项说明

[配置文件样例](config.example.yaml)

> [!TIP]
> 在配置文件中，字符开头为 `#?` 的配置项代表**尚未开发/暂未启用**的功能。<br>建议先下载样例再慢慢修改.

#### 1. 顶层配置

| 配置项   | 类型   | 备注         |
| -------- | ------ | ------------ |
| pid_file | string | PID 文件路径 |

> [!TIP]
> `pid_file` 在非 Windows 平台默认会写入 `/var/run/mc-gateway.pid`，<br>在 Windows 平台默认不会写入任何文件。

#### 2. hosts

hosts 使用期望的 host 作为键（Key），转发的目的地址作为值（Value）。
* 默认的 fallback host 配置 Key 值为 `default`。
* 支持 `haproxy://` 等协议代理格式（具体参考配置示例）。

#### 3. log (日志)

| 配置项 | 类型   | 备注                                            |
| ------ | ------ | ---                                             |
| level  | Level  | 日志等级（如 `warn`, `info`, `error`, `debug`） |
| file   | string | 日志文件输出路径                                |

> [!TIP]
> 日志适配了 `logrotate`，可以使用 `logrotate` 进行日志分片、压缩等日常运维操作。<br>参考配置如下：

```logrotate
/var/log/mc-gateway.log {
    copytruncate
    daily
    missingok
    rotate 14
    compress
    compresscmd /usr/bin/zstd
    compressext .zst
    compressoptions -T0 --long
    uncompresscmd /usr/bin/unzstd
    notifempty
    delaycompress
    dateext
    postrotate
        # 向程序发送 SIGHUP 信号
        # mc-gateway 默认会将当前进程的 pid 写入 /var/run/mc-gateway.pid
        if [ -f /var/run/mc-gateway.pid ]; then
            kill -SIGHUP $(cat /var/run/mc-gateway.pid)
        fi
    endscript
}

```

#### 4. tcp

| 配置项 | 类型 | 默认  | 备注              |
| ------ | ---- | ----- | ----------------- |
| enable | bool | false | 是否启用 TCP 服务 |
| port   | int  | 25565 | 监听端口          |

> [!TIP]
> 默认端口为 `25565`，与 Minecraft 官方服务端保持一致。

#### 5. websocket

| 配置项          | 类型   | 默认   | 备注                                              |
| --------------- | ------ | ------ | ------------------------------------------------- |
| enable          | bool   | false  | 是否启用 WebSocket 服务                           |
| port            | int    | 8080   | 监听端口                                          |
| path            | string | /      | 接口路径                                          |
| trust_ip_header | string | 无     | *[未开发]* 信任的真实 IP 请求头（如 `X-Real-IP`） |

> [!TIP]
> `path` 默认为 `"/"`，会对所有路径的请求进行处理。

#### 6. quic

| 配置项          | 类型     | 备注                                              |
| --------------- | -------- | ------------------------------------------------- |
| enable          | bool     | 是否启用 QUIC 服务                                |
| port            | int      | 监听端口                                          |
| alpn            | []string | 应用层协议协商列表（ALPN）                        |
| trust_ip_header | string   | *[未开发]* 信任的真实 IP 请求头（如 `X-Real-IP`） |

> [!TIP]
> `alpn` 只要客户端与服务端有一个能够对应上即可成功连接。<br>默认值为 `["minecraft", "quic", "raw", "h3"]`。

#### 7. kcp

| 配置项        | 类型 | 备注                    |
| ------------- | ---- | ----------------------- |
| enable        | bool | 是否启用 KCP 服务       |
| port          | int  | 监听端口                |
| data_shards   | int  | Reed-Solomon 数据分片数 |
| parity_shards | int  | Reed-Solomon 校验分片数 |