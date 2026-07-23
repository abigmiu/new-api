# 上游首字延迟排查

## 目的

定位流式请求的首字延迟发生在客户端、Nginx、newapi 还是上游。

## 计时口径

```text
客户端 -> Nginx -> newapi -> 上游
                  |          |
              StartTime    上游请求开始
                             |
                         响应头 / 首个 SSE 事件
```

- `frt`：newapi 收到第一个上游 SSE 事件减去 `StartTime`，单位毫秒。
- `use_time`：从 `StartTime` 到流结束，单位秒。
- 上游账单的 First Token 和 Duration 从上游侧计时，不能直接等同于 newapi 的 `frt` 和 `use_time`。

## 全渠道追踪

所有渠道的上游请求都会记录请求开始和响应头；流式请求额外记录首个 SSE 事件：

```text
upstream request start: channel=29 retry=0 inbound_to_upstream_ms=...
upstream response headers: channel=29 retry=0 elapsed_ms=... status=200
upstream first event: channel=29 retry=0 headers_to_first_event_ms=... request_to_first_event_ms=...
```

含义：

| 字段 | 阶段 |
| --- | --- |
| `inbound_to_upstream_ms` | newapi 接收请求到调用上游 HTTP 客户端前的耗时 |
| `elapsed_ms` | 上游 HTTP 请求开始到收到响应头的耗时 |
| `headers_to_first_event_ms` | 收到响应头到第一个 SSE 事件的耗时 |
| `request_to_first_event_ms` | 上游 HTTP 请求开始到第一个 SSE 事件的耗时 |

## Nginx 访问日志

API 站点使用 `timing` 日志格式时，确认日志存在以下字段：

```text
rt=$request_time
uct=$upstream_connect_time
uht=$upstream_header_time
urt=$upstream_response_time
rl=$request_length
us=$upstream_status
```

- `uht` 大：请求已进入 newapi，但首个响应迟迟未返回 Nginx；继续看 newapi 的三段上游日志。
- `uct` 大：Nginx 到 newapi 的本机连接异常。
- `uht` 小但客户端感知慢：排查客户端、Cloudflare 或客户端网络。
- `rl` 很大：结合 `inbound_to_upstream_ms` 判断是否为请求体读取、解析或本地计费阶段造成延迟。

## 排查顺序

1. 从使用日志筛选 `frt >= 30000` 的请求，记录请求 ID、渠道和结束时间。
2. 查同一请求 ID 的 newapi 日志：

   ```bash
   grep '<request-id>' /app/logs/oneapi-*.log
   ```

3. 查同一时间点的 Nginx access log：

   ```bash
   grep 'POST /v1/responses' /www/sites/api.wewont.top/log/access.log | tail -n 50
   ```

4. 按下表归因：

| 现象 | 结论 |
| --- | --- |
| `inbound_to_upstream_ms` 很大 | newapi 在请求解析、鉴权、计费、渠道选择或构造上游请求前耗时过长 |
| `elapsed_ms` 很大 | 到上游的连接、请求上传或上游接入队列异常 |
| `headers_to_first_event_ms` 很大 | 上游模型排队或生成首字慢 |
| 三项都小但 `uht` 大 | 检查 SSE 转发和 Nginx 配置 |
| `uht` 小但客户端慢 | 检查 Cloudflare、客户端网络和客户端消费流的速度 |

## 本次渠道 29 基线

2026-07-22 至 2026-07-23 的逐条对齐显示：

- 上游 First Token 平均约 3.2 秒；newapi `frt` 平均约 7.6 秒。
- 多数总用时差来自首字后的持续生成，不是故障。
- `frt` 超过 60 秒的异常请求中，多数上游账单的 First Token 仅 1 至 3 秒，需用本追踪确认延迟发生在 newapi 发起上游请求前还是上游接入前。

## 恢复默认

日志是全渠道常驻诊断信息，无需额外环境变量。保留 Nginx timing 日志可用于后续轻量排查。

## 构建、验证与回滚

构建镜像后，先启动一个临时容器或在测试环境发送一条流式请求。确认同一个请求 ID 出现以下顺序的日志后，再替换生产容器：

```text
upstream request start
upstream response headers
upstream first event
```

生产替换前记录当前镜像 ID。若新镜像启动、健康检查或转发异常，立即恢复该镜像并重启容器。Nginx 的 `timing` 日志配置与 newapi 镜像独立，不需要随 newapi 回滚。
