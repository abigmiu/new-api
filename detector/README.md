# gpt5.6 渠道定时检测器（独立容器）

对 new-api 里**所有启用渠道**做定时检测，判断每个渠道是否路由真实 GPT-5.6 模型。

- **独立容器**：与本项目完全隔离，不共用镜像/依赖/端口，不会冲突。
- **CI 自动打包**：推送到 GitHub 后 Actions 自动构建多架构镜像到 `ghcr.io`，服务器只需 `docker compose` 拉镜像运行，无需源码构建环境。
- **每次运行前实时同步渠道**：先调管理端 API 拉取最新启用渠道，不缓存。
- **Token 硬固定**：用 `sk-<key>-<channelId>` 精确路由到指定渠道，失败重试也不串线。
- **只存数据，不存 HTML**：检测原始 JSON 为唯一存档；展示由 Web 服务**实时动态渲染**。
- **对外用渠道偏好同款代号（alias）**，客户看到的代号与「渠道偏好」页一致，**绝不暴露真实渠道名**。
- **公网可访问**：容器内动态展示服务，端口映射 + 反向代理暴露公网。

## 原理

### 逐个检测且不串线
new-api 的 token 鉴权支持在 key 后追加渠道 ID：`Authorization: Bearer sk-<tokenKey>-<channelId>`。
`middleware/auth.go` 解析后缀 → `middleware/distributor.go` 直接选用该渠道（不经过随机/偏好），
且重试时也停在原渠道。因此只需**一个 admin 账户的 API token**，对每个渠道拼不同的渠道 ID 即可。

### 代号（alias）与渠道偏好一致
new-api 的「渠道偏好」页用 `CHANNEL_ALIAS_KEY`（FF1）给每个渠道生成 6 位代号（如 `YYSJTN`），
按分组展示为 `group-alias`。为了让本系统展示**同一套代号**，本项目修改了 new-api 一个接口：

- `controller/channel_preference.go`：`GetChannelPreferences` 对**管理员**额外返回 `channel_id` 字段
  （非管理员不返回，`omitempty` 隐藏）。
- 检测器每轮从该接口拉取 `channel_id ↔ alias` 映射，存储为 `reports/aliases.json`，Web 端用它展示。
- 客户在「渠道偏好」页和本检测页面看到的渠道代号完全一致；真实渠道名只存在于
  `reports/owner-names.json`（管理员本地查看用，Web 服务不对外提供）。

## 目录结构

```
detector/
  gpt56/            # gpt56 检测器（已 vendor，纯标准库）
  scheduler.py      # 每周期：同步渠道+别名 → 逐渠道检测（数据入库）→ 删 HTML 只留数据
  data.py           # 数据处理：读取原始 JSON，加工成展示数据集（只含 alias）
  web.py            # 动态展示服务：总览/渠道历史/全局历史/数据API，实时渲染
  runner.py         # 容器主进程：调度循环 + 动态展示服务
  Dockerfile
  docker-compose.yml
  .env.example
```

## 部署步骤（CI 镜像方案，推荐）

> **前置：需要把 new-api 源码更新到含 `channel_id` 字段的版本并重新部署**，否则别名无法与
> 「渠道偏好」页对齐（本仓库已改好，见下方「对 new-api 的修改」）。

### A. 首次构建镜像（只在你的开发机上做一次）

1. 把代码推到你的 GitHub 仓库（含 `detector/**`）。CI（`.github/workflows/gpt56-detector.yml`）
   会在 main 分支对 `detector/` 的改动自动构建 `ghcr.io/<user>/new-api/gpt56-detector:latest`
   （多架构 amd64/arm64），并支持 `gpt56-*` tag 触发 `docker tag` 镜像。
2. 确认 Actions 构建成功（仓库页面 → Actions → **Build gpt56-detector image**）。首次构建前
   需把 GHCR 设为公开，或在服务器上先 `docker login ghcr.io` 登录。

### B. 在服务器上运行

把仓库根的 **`compose.gpt56.yml`**（拉镜像版，无需构建）+ 一份 `.env` 放到服务器任意目录：

```bash
cd /opt/gpt56-detector
# 把 compose.gpt56.yml 拷到这里并重命名，首次生成 .env 样例：
cp compose.gpt56.yml compose.yml
cp detector/.env.example .env && vi .env   # 只填必需项（见下方“配置项”）
mkdir -p reports

# 拉取并启动（服务器需要有 docker compose v2）
docker compose up -d
```

访问 `http://<服务器IP>:8080/`。改配置 / 升级镜像：

```bash
docker compose restart                # 改 .env 后重启
docker compose pull && docker compose up -d   # 升级到最新镜像
```

> 也可 `${REPORT_PUBLIC_PORT}` 改端口；建议用 Nginx/Caddy 反代该端口。

### C. 本地背书：推送 GitHub 时自动触发

平常改动 `detector/` 后按正常 git 推送即可，CI 会自动重建镜像。tag 触发：`git tag gpt56-<版本>`。

---

## 旧：纯本地构建（不用 CI）

开发机打包并上传，服务器本地 `docker build`。不依赖 GitHub，但服务器需安装完整构建工具链：

```bash
./detector/deploy.sh <user@host> /opt/gpt56-detector
```

## 对 new-api 的修改

文件：`controller/channel_preference.go`

```go
type channelPreferenceOption struct {
    Alias       string `json:"alias"`
    DisplayName string `json:"display_name"`
    ChannelID   int    `json:"channel_id,omitempty"`   // 新增：仅管理员可见
}
```

`GetChannelPreferences` 中按角色填充：

```go
isAdmin := c.GetInt("role") >= common.RoleAdminUser
// 循环内：
if isAdmin {
    option.ChannelID = channel.Id
}
```

- 非管理员请求该接口不会收到 `channel_id` 字段（`omitempty`）。
- 已附测试 `TestGetChannelPreferencesChannelIDVisibleOnlyToAdmin`。

## 配置项（.env）

| 变量 | 说明 |
|---|---|
| `GATEWAY_BASE_URL` | 网关 relay 基址，如 `https://gw.example.com/v1` |
| `NEWAPI_ADMIN_BASE_URL` | 网关管理端基址（根路径） |
| `NEWAPI_ADMIN_ACCESS_TOKEN` | 管理员 PAT |
| `DETECTOR_API_KEY` | 管理员 token 原始 key（不含 `sk-`） |
| `DETECT_MODE` | `juice`（默认）/ `full`（完整 COT+Juice，需可信端） |
| `TRUSTED_BASE_URL/MODEL/API_KEY` | `full` 模式可信端 |
| `MODEL_PATTERN` | 从渠道 Models 挑模型的正则，默认 `gpt-5\.6` |
| `FALLBACK_MODEL` | 渠道未配置 Models 时的模型 |
| `DETECT_INTERVAL_MINUTES` | 检测间隔分钟，默认 `60` |
| `DETECT_CONCURRENCY` | 渠道间并发数（默认 1，1~3 为宜，避免触发限流） |
| `DETECT_WORKERS` / `DETECT_TIMEOUT` | 渠道内请求并发 / 单请求超时 |
| `REPORT_PUBLIC_PORT` | 公网端口映射 |
| `HTTP_USER_AGENT` / `GPT56_USER_AGENT` | 浏览器 UA（正式服若在 Cloudflare 后必需） |

## 数据与页面

- 每渠道每次检测生成 `reports/ch<id>-<时间戳>.json`（唯一存档，文件名不含渠道名）+ `.log`。
  探针自带的 `.html` 会在检测后删除。
- Web 端（动态渲染）：
  - `/`：总览（各渠道代号、最新结论、型号、线路、通过率、检测次数）。
  - `/channel/<alias>`：单渠道历史。
  - `/history`：全部历史。
  - `/api/data.json`：加工后的数据 API（前端/集成用，只含 alias）。
- 检测期间渠道 API 会被真实调用（计费），请给测试 token 预留充足额度。
