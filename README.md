# Fund Tracking

Fund Tracking 是一个已上线的基金信息跟踪系统，用于展示基金列表、基金详情、历史净值快照、趋势图和用户关注列表。项目不提供投资建议、交易、下单或支付能力。

当前项目重点是 Go 后端工程实践：使用 Gin 提供 REST API，MongoDB Atlas 作为数据源，Redis 做缓存与短期任务状态，RabbitMQ 承载异步更新任务，并通过 Nginx、systemd 和 GitHub Actions 组成线上运维链路。

## 在线访问

| 项目 | 地址 |
| --- | --- |
| 前端网站 | [https://www.fundtracking.online](https://www.fundtracking.online) |
| 后端 API | [https://api-fund.fundtracking.online](https://api-fund.fundtracking.online) |
| 健康示例 | [https://api-fund.fundtracking.online/api/version](https://api-fund.fundtracking.online/api/version) |

## 核心功能

- 基金数据展示、基金详情页和基金搜索
- 基金历史快照与 7d / 30d 趋势图
- watchlist 关注列表
- 购买日期、购买净值、当前净值、持有收益率计算
- 邮箱验证码注册与 JWT 登录认证
- Redis 缓存基金详情、watchlist 和异步任务状态
- RabbitMQ 异步更新基金数据
- 异步任务状态轮询
- GitHub Actions 定时触发基金数据更新和提醒任务

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 前端 | Next.js / React / Vercel |
| 后端 | Go / Gin |
| 数据库 | MongoDB Atlas |
| 缓存与任务状态 | Redis |
| 消息队列 | RabbitMQ |
| 认证 | JWT / bcrypt |
| 邮件 | Resend |
| 部署 | 阿里云 ECS Ubuntu 22.04 / Nginx / systemd |
| HTTPS | Certbot DNS-01 |
| 定时任务 | GitHub Actions |

## 系统架构

```text
Browser
  -> Vercel Frontend
  -> https://api-fund.fundtracking.online
  -> Nginx 80/443
  -> 127.0.0.1:8081
  -> Go Gin API
  -> MongoDB Atlas / Redis / RabbitMQ / Resend
```

线上部署状态：

- 前端部署在 Vercel，生产环境变量 `NEXT_PUBLIC_GO_API_URL=https://api-fund.fundtracking.online`。
- 后端部署在阿里云 ECS Ubuntu 22.04。
- Go 后端通过 Windows 本地交叉编译为 Linux amd64 二进制后上传服务器。
- 后端服务路径：`/opt/fund-tracking/backend-go/fund-tracking-server`。
- systemd 服务名：`fund-tracking.service`。
- Nginx 监听 `80/443`，HTTPS 反向代理到 `127.0.0.1:8081`。
- MongoDB Atlas 保存业务数据，Redis 保存缓存和任务状态，RabbitMQ 负责异步任务分发。

## 异步更新链路

生产环境中，公开前端不会直接持有 `UPDATE_API_KEY`。浏览器调用 client-safe 路由，Nginx 在服务端注入 `X-Update-Key` 后转发给 Go 后端。

```text
Browser
  -> POST https://api-fund.fundtracking.online/api/update/async-client
  -> Nginx injects X-Update-Key
  -> Go Backend POST /api/update/async
  -> RabbitMQ publishes task
  -> Worker consumes task
  -> Redis stores task status
  -> MongoDB Atlas updates fund data
  -> Browser polls /api/update/tasks-client/{task_id}
```

后端内部处理：

```text
POST /api/update/async
  -> 校验 X-Update-Key
  -> 创建 task_id
  -> Redis 写入 pending
  -> 发布 RabbitMQ persistent message
  -> Worker 更新状态为 running
  -> 获取 Redis update lock
  -> 抓取基金数据并写入 MongoDB Atlas
  -> 删除相关 Redis 缓存
  -> Redis 写入 success / failed
  -> RabbitMQ ack
```

实现细节：

- RabbitMQ 队列默认名：`fund.update.tasks`。
- 当前 worker 与 Go API 在同一个进程内启动，后续可拆成独立 worker 服务。
- Redis task key 示例：`fund:update:task:{taskID}`，TTL 为短期运行态数据。
- Redis update lock 用于避免同步和异步更新同时写入基金数据。
- RabbitMQ 消息发布失败时，Redis task 会标记为 `failed`。

## 数据模型简述

| 数据 | 说明 |
| --- | --- |
| fund data | 基金基础信息、最新净值、更新时间和阶段表现字段 |
| fund history | 按基金代码保存历史净值快照，用于详情页和趋势图 |
| users | 邮箱、密码哈希、邮箱验证状态等认证数据 |
| watchlists | 用户关注基金、提醒阈值、购买日期、购买净值和收益率相关字段 |
| alert logs | 提醒检查与发送记录，避免重复发送 |
| Redis task status | 异步更新任务的 pending / running / success / failed 状态 |

MongoDB Atlas 是长期数据源；Redis 只保存缓存、锁和短生命周期任务状态。

## 本地开发

### 1. 启动依赖

```bash
docker run --name fundtracking-redis -p 6379:6379 -d redis:7-alpine
```

```bash
docker run -d --name fundtracking-rabbitmq \
  --hostname fundtracking-rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  rabbitmq:4-management
```

可选本地 MongoDB：

```bash
docker run -d --name fundtracking-mongo -p 27017:27017 mongo:7
```

### 2. 启动 Go 后端

```bash
cd backend-go
go run .
```

默认监听 `127.0.0.1:8081`，可通过 `PORT` 修改。

### 3. 启动前端

```bash
cd client
npm install
npm run dev
```

前端本地示例：

```text
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
```

## 环境变量说明

以下只展示变量名和占位示例，不包含任何真实线上密钥。

### 后端

```text
MONGO_URI=mongodb+srv://<user>:<password>@<cluster>/<db>?retryWrites=true&w=majority
JWT_SECRET=replace-with-a-strong-secret
UPDATE_API_KEY=replace-with-server-side-update-key
RESEND_API_KEY=replace-with-resend-api-key
ALERT_EMAIL_FROM=noreply@example.com
ALERT_EMAIL_FROM_NAME=Fund Tracking

REDIS_URL=
REDIS_ADDR=127.0.0.1:6379
RABBITMQ_URL=amqp://<user>:<password>@127.0.0.1:5672/
RABBITMQ_UPDATE_QUEUE=fund.update.tasks

APP_VERSION=local-dev
APP_BUILT_AT=
GIT_COMMIT=
SOURCE_VERSION=
PORT=8081
```

### 前端

```text
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
NEXT_PUBLIC_APP_VERSION=local-dev
NEXT_PUBLIC_SHOW_VERSION_BADGE=false
```

### GitHub Actions

```text
BACKEND_BASE_URL=https://api-fund.fundtracking.online
UPDATE_API_KEY=<stored-in-github-secrets>
```

安全约束：

- 不要提交真实 `MONGO_URI`、`JWT_SECRET`、`UPDATE_API_KEY`、`RESEND_API_KEY`。
- `NEXT_PUBLIC_*` 变量会暴露给浏览器，只能放公开配置。
- 生产环境由 Nginx 注入 `X-Update-Key`，前端不保存更新密钥。
- 线上 RabbitMQ 不使用默认 `guest/guest`。

## 部署说明

### 前端

前端部署在 Vercel，生产环境设置：

```text
NEXT_PUBLIC_GO_API_URL=https://api-fund.fundtracking.online
```

### 后端

Windows 本地交叉编译 Linux amd64：

```powershell
cd backend-go
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o fund-tracking-server .
```

上传到服务器路径：

```text
/opt/fund-tracking/backend-go/fund-tracking-server
```

systemd 托管服务：

```bash
sudo systemctl status fund-tracking.service
sudo systemctl restart fund-tracking.service
journalctl -u fund-tracking.service -f
```

Nginx 负责：

- HTTPS 终止和证书管理。
- 将 `https://api-fund.fundtracking.online/api/*` 反向代理到 `127.0.0.1:8081`。
- 对 `/api/update/async-client` 和 `/api/update/tasks-client/{task_id}` 注入 `X-Update-Key` 后转发到后端真实维护接口。

证书通过 Certbot DNS-01 签发，避免在 README 中记录任何 DNS 或 API 密钥。

### 定时更新

`.github/workflows/trading_update.yml` 在工作日定时运行，通过 GitHub Secrets 中的 `BACKEND_BASE_URL` 和 `UPDATE_API_KEY` 调用维护接口：

- `POST /api/update`
- `GET /api/alerts/check`
- `POST /api/alerts/send`

## 接口示例

### 基础接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/version` | 服务版本 |
| GET | `/api/health/mongo` | MongoDB 健康检查 |
| GET | `/api/funds` | 基金列表 |
| GET | `/api/fund/:code` | 基金详情 |
| GET | `/api/funds/:code/history?range=7d` | 历史净值快照 |
| GET | `/api/funds/search?query=` | 基金搜索 |

### 认证与用户

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/register` | 注册并发送邮箱验证码 |
| POST | `/api/auth/verify-email-code` | 校验邮箱验证码 |
| POST | `/api/auth/resend-email-code` | 重发邮箱验证码 |
| POST | `/api/auth/login` | 登录并返回 JWT |
| GET | `/api/auth/me` | 获取当前用户 |

### Watchlist

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/watchlist` | 获取关注列表 |
| POST | `/api/watchlist` | 添加关注基金 |
| PUT | `/api/watchlist/:fundCode` | 更新提醒阈值、购买日期等信息 |
| DELETE | `/api/watchlist/:fundCode` | 删除关注基金 |

### 维护接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/update` | 同步更新基金数据 |
| POST | `/api/update/async` | 创建异步更新任务 |
| GET | `/api/update/tasks/:id` | 查询异步任务状态 |
| POST | `/api/funds/enrich` | 补充基金元数据 |
| POST | `/api/funds/performance` | 补充阶段表现字段 |
| GET | `/api/alerts/check` | 检查关注基金提醒 |
| POST | `/api/alerts/send` | 发送提醒邮件 |

维护接口需要 `X-Update-Key`。浏览器侧使用 `async-client` / `tasks-client` 路径，由 Nginx 注入密钥后转发，不直接暴露 `UPDATE_API_KEY`。

### 异步任务示例

```bash
curl -X POST http://127.0.0.1:8081/api/update/async \
  -H "X-Update-Key: replace-with-local-update-key"
```

```json
{
  "status": "accepted",
  "task_id": "update_xxx"
}
```

```bash
curl http://127.0.0.1:8081/api/update/tasks/update_xxx \
  -H "X-Update-Key: replace-with-local-update-key"
```

```json
{
  "id": "update_xxx",
  "status": "success",
  "response": {
    "status": "success",
    "updated": 10,
    "failed": [],
    "total": 10
  }
}
```

## 面试亮点

- 后端从历史 Python 实现逐步迁移到 Go Gin，保持前端 API 路径和 JSON 结构兼容。
- 用 MongoDB Atlas 承载基金、用户、关注列表和历史快照数据，更新任务使用 upsert 保持幂等。
- Redis 同时处理 cache aside、watchlist 缓存、任务状态和分布式更新锁。
- RabbitMQ 将前端触发和真实基金更新解耦，接口可快速返回 `task_id`，前端再轮询状态。
- Nginx 在服务端注入 `X-Update-Key`，避免把维护接口密钥暴露给公开前端。
- Go 二进制由 systemd 托管，Nginx 做 HTTPS 反代，生产链路接近真实后端服务部署。
- GitHub Actions 定时调用维护接口，配合 GitHub Secrets 管理敏感配置。
- Resend 支持注册验证码和提醒邮件，业务密钥只保存在服务端环境。

## 后续优化方向

- 将当前进程内 worker 拆分为独立 worker 服务。
- 增加 RabbitMQ publisher confirm、DLQ 和有限重试策略。
- 为异步任务提供更细粒度进度字段。
- 补充更多 Go handler / service 层测试。
- 继续按模块迁移和整理历史后端参考代码，保持前端兼容。

## 项目边界

- 项目只做基金信息展示、跟踪和关注管理。
- 项目不提供投资建议、交易、下单或支付功能。
- 真实密钥只应保存在服务器环境变量、Nginx 配置、Vercel 环境变量或 GitHub Secrets 中。
