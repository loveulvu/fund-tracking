# Fund Tracking

Fund Tracking 是一个基金跟踪项目，前端提供基金列表、搜索、详情、登录注册和 watchlist 管理，后端提供基金数据读取、用户认证、关注列表、行情更新和提醒任务接口。项目当前主链路为 Next.js 前端、Go `net/http` 后端、MongoDB Atlas 数据库，并通过 GitHub Actions 定时触发后端更新任务。

本仓库保留了渐进式 Go 后端迁移的痕迹：当前线上核心 API 由 `backend-go` 提供，旧后端代码不再作为主运行链路。README 内容基于当前仓库结构、Go 路由、Dockerfile、`.dockerignore` 和 GitHub Actions workflow 整理。

## 项目结构

```text
.
├── backend-go/                 # Go 后端，net/http + MongoDB Go Driver
│   ├── main.go                 # 服务启动、Mongo 初始化、路由注册
│   ├── update.go               # /api/update 行情更新
│   ├── auth.go                 # 注册、登录、JWT、邮箱验证码
│   ├── watchlist.go            # watchlist 增删查改
│   ├── alerts.go               # 阈值检查与邮件提醒
│   ├── import.go               # 未收录基金导入
│   ├── enrich.go               # 基金元数据补全
│   ├── performance.go          # 阶段收益补全
│   ├── Dockerfile              # Go 后端多阶段 Docker 构建
│   └── .dockerignore           # Docker build context 排除规则
├── client/                     # Next.js 前端
│   └── src/
├── .github/workflows/
│   └── trading_update.yml      # 工作日定时触发 update/check/send
├── docs/                       # 项目补充文档
└── README.md
```

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 前端 | Next.js 16、React 19、TypeScript 配置、CSS/Tailwind 相关配置 |
| 后端 | Go 1.26、标准库 `net/http`、MongoDB Go Driver |
| 数据库 | MongoDB Atlas |
| 认证 | JWT、bcrypt、邮箱 6 位验证码 |
| 自动化 | GitHub Actions 定时 workflow |
| 部署 | Vercel 前端、Render Go 后端 |
| 容器化 | Docker，多阶段构建 |

## 核心功能

- 基金列表展示：`GET /api/funds`
- 基金搜索：`GET /api/search_proxy?query=...`，同时保留 `GET /api/funds/search?query=...`
- 基金详情：`GET /api/fund/{fundCode}`
- 登录注册：邮箱注册、登录、JWT 鉴权、邮箱验证码验证和重发
- Watchlist：登录用户可以添加、查看、删除关注基金，并修改提醒阈值
- 未收录基金导入：登录后可导入合法的 6 位基金代码
- 定时更新：GitHub Actions 工作日触发基金行情更新、阈值检查和提醒邮件发送
- 数据补全：后端包含基金元数据补全和阶段收益补全维护接口

## Go 后端接口概览

当前 Go 服务在 `backend-go/main.go` 中直接使用 `http.HandleFunc` 注册路由。

| 方法 | 路径 | 说明 | 鉴权 |
| --- | --- | --- | --- |
| GET | `/api/version` | 返回服务名、版本、commit、构建时间和服务器时间 | 无 |
| GET | `/api/health/mongo` | MongoDB 连接健康检查 | 无 |
| GET | `/api/funds` | 返回基金列表 | 无 |
| GET | `/api/fund/{fundCode}` | 返回单只基金详情 | 无 |
| GET | `/api/search_proxy?query=...` | 基金代码/名称搜索，兼容前端旧路径 | 无 |
| GET | `/api/funds/search?query=...` | 基金代码/名称搜索 | 无 |
| POST | `/api/auth/register` | 注册并发送邮箱验证码 | 无 |
| POST | `/api/auth/login` | 登录并返回 JWT | 无 |
| POST | `/api/auth/verify-email-code` | 验证邮箱 6 位验证码 | 无 |
| POST | `/api/auth/resend-email-code` | 重发邮箱验证码 | 无 |
| GET | `/api/auth/me` | 返回当前登录用户信息 | JWT |
| GET | `/api/watchlist` | 获取当前用户 watchlist | JWT |
| POST | `/api/watchlist` | 添加 watchlist 项 | JWT |
| PUT | `/api/watchlist/{fundCode}` | 修改提醒阈值 | JWT |
| DELETE | `/api/watchlist/{fundCode}` | 删除 watchlist 项 | JWT |
| POST | `/api/funds/import` | 导入未收录的 6 位基金代码 | JWT |
| GET | `/api/update` | 更新默认基金和 watchlist 中基金的基础行情 | `X-Update-Key` |
| POST | `/api/funds/enrich` | 补全基金类型、公司、经理、规模等元数据 | `X-Update-Key` |
| POST | `/api/funds/performance` | 补全阶段收益字段 | `X-Update-Key` |
| GET | `/api/alerts/check` | 扫描 watchlist 阈值并写入提醒日志 | `X-Update-Key` |
| POST | `/api/alerts/send` | 发送待处理提醒邮件 | `X-Update-Key` |

## `/api/update` 并发更新设计

`/api/update` 的实现位于 `backend-go/update.go`，用于更新基金基础行情数据。

更新目标来源：

```text
defaultFundCodes + distinct(watchlists.fundCode)
```

处理流程：

1. 校验请求方法为 `GET`。
2. 通过请求头 `X-Update-Key` 和环境变量 `UPDATE_API_KEY` 做后台任务鉴权。
3. 基于 `r.Context()` 创建 30 秒超时 context：`context.WithTimeout(r.Context(), 30*time.Second)`。
4. 汇总默认基金代码和 MongoDB `watchlists` 集合中的基金代码。
5. 只接受 6 位数字基金代码，非法代码进入 `skipped_codes`。
6. 使用 worker pool 并发更新，当前 worker 数为 `3`。
7. 每个 worker 调用外部基金行情接口，使用 `http.NewRequestWithContext(ctx, ...)` 绑定同一个超时 context。
8. 更新 MongoDB 时继续传递同一个 `ctx`，例如 `collection.UpdateOne(ctx, ...)`。
9. worker 只把 `updateFundResult` 写入 channel，不直接修改共享统计切片。
10. 主 goroutine 统一读取结果、统计成功/失败、补齐超时未返回的基金代码，避免 data race。

响应字段包括：

```text
status
updated
failed
total
duration_ms
target_codes
updated_codes
failed_codes
skipped_codes
```

`status` 可能为：

- `success`：全部成功。
- `partial_success`：部分成功、部分失败。
- `failed`：没有成功更新且存在失败项。

## Docker 化说明

`backend-go/Dockerfile` 使用多阶段构建：

1. `golang:1.26-alpine AS builder`：下载 Go module，编译静态 Linux 可执行文件。
2. `alpine:3.22`：作为运行镜像，只复制编译后的二进制文件。
3. 运行阶段创建非 root 用户 `appuser`。
4. 默认设置 `PORT=8081` 并暴露 `8081`。

`backend-go/.dockerignore` 会排除 `.git`、`.github`、`.env`、`.env.*`、日志、临时目录、测试二进制等内容，减少 build context，并避免把本地环境文件复制进镜像。

镜像和容器的关系：

- 镜像是构建产物，可以理解为应用运行所需文件和运行环境的模板。
- 容器是镜像启动后的运行实例。
- `.env.docker` 不会被打进镜像；本地运行容器时通过 `--env-file` 从宿主机注入环境变量。

本地构建镜像：

```bash
docker build -t fund-tracking-go-api ./backend-go
```

本地启动容器：

```bash
docker run --rm \
  --env-file backend-go/.env.docker \
  -p 8081:8081 \
  fund-tracking-go-api
```

Windows PowerShell 可以写成一行：

```powershell
docker run --rm --env-file backend-go/.env.docker -p 8081:8081 fund-tracking-go-api
```

注意：`.env.docker` 应只放本地开发值或测试值，不应放生产 secret。当前 `.dockerignore` 已排除 `.env.*`，但通过 `--env-file` 传入的值仍会进入正在运行的容器环境。

## 本地开发运行

### 后端

Go 后端不会自动读取 `.env` 文件，需要在 shell 或运行平台中提供环境变量。

必需环境变量：

| 变量 | 说明 |
| --- | --- |
| `MONGO_URI` | MongoDB 连接地址 |
| `JWT_SECRET` | JWT 签名密钥 |
| `UPDATE_API_KEY` | 保护 `/api/update`、`/api/alerts/*`、补全接口的后台密钥 |

可选环境变量：

| 变量 | 说明 |
| --- | --- |
| `PORT` | 后端监听端口，默认 `8081` |
| `APP_VERSION` | `/api/version` 返回的版本标识 |
| `APP_BUILT_AT` | `/api/version` 返回的构建时间 |
| `GIT_COMMIT` / `SOURCE_VERSION` | `/api/version` 返回的 commit 信息 |
| `RESEND_API_KEY` | 发送提醒邮件和验证码邮件所需 |
| `ALERT_EMAIL_FROM` | 提醒邮件发件地址 |
| `ALERT_EMAIL_FROM_NAME` | 提醒邮件发件名称 |

启动后端：

```bash
cd backend-go
go run .
```

健康检查：

```bash
curl http://127.0.0.1:8081/api/version
curl http://127.0.0.1:8081/api/health/mongo
```

触发受保护的更新接口：

```bash
curl -H "X-Update-Key: $UPDATE_API_KEY" \
  http://127.0.0.1:8081/api/update
```

### 前端

前端位于 `client`，通过 `NEXT_PUBLIC_GO_API_URL` 指向 Go 后端。示例见 `client/.env.example`。

```bash
cd client
npm install
npm run dev
```

默认本地后端地址：

```text
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
```

## 部署说明

### Vercel 前端

前端部署在 Vercel，构建目录为 `client`。需要配置公开环境变量：

| 变量 | 说明 |
| --- | --- |
| `NEXT_PUBLIC_GO_API_URL` | Render 上 Go 后端的公开访问地址 |

### Render 后端

Go 后端部署在 Render，运行 `backend-go` 服务。生产环境至少需要配置：

| 变量 | 说明 |
| --- | --- |
| `MONGO_URI` | MongoDB Atlas 连接地址 |
| `JWT_SECRET` | JWT 签名密钥 |
| `UPDATE_API_KEY` | 后台任务接口密钥 |
| `RESEND_API_KEY` | 邮件发送 API key |
| `ALERT_EMAIL_FROM` | 提醒邮件发件地址 |
| `ALERT_EMAIL_FROM_NAME` | 提醒邮件发件名称 |
| `APP_VERSION` | 服务版本标识 |

### GitHub Actions 定时任务

`.github/workflows/trading_update.yml` 在 `main` 分支上通过 schedule 和手动触发执行。当前定时配置为：

```yaml
cron: "0 11 * * 1-5"
```

这对应 UTC 11:00，约为北京时间/日本时间工作日 20:00。

workflow 调用顺序：

```text
GET  /api/update
GET  /api/alerts/check
POST /api/alerts/send
```

每一步都会带上：

```text
X-Update-Key: ${{ secrets.UPDATE_API_KEY }}
```

GitHub Actions 需要配置的 Secrets：

| Secret | 说明 |
| --- | --- |
| `BACKEND_BASE_URL` | Render Go 后端公开地址 |
| `UPDATE_API_KEY` | 与 Render 后端环境变量一致的后台任务密钥 |

workflow 会上传以下响应文件作为 artifact，保留 7 天：

```text
update-response.json
alerts-check-response.json
alerts-send-response.json
```

## 项目边界

- 这是基金数据展示、关注和提醒项目，不是交易系统。
- 当前没有描述为分布式系统或微服务架构。
- `/api/update` 使用固定 worker pool 做有限并发，目标是缩短批量更新耗时并保持实现简单。
- MongoDB schema 未在 README 中要求或假设发生变更。
- 生产 secret 不应写入 README、前端公开环境变量或提交到仓库。
