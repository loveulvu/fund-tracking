# Fund Tracking 基金信息追踪系统

## 项目简介

Fund Tracking 是一个基金信息追踪系统，用于展示和追踪基金信息，不提供投资建议或交易能力。

项目前端使用 Next.js / React，后端使用 Go + Gin，支持基金列表、基金详情、基金搜索、自选基金、用户登录鉴权，以及基金数据的同步和异步更新。

项目重点是 Go 后端工程实践，包括 REST API、MongoDB 数据访问、Redis 缓存、Redis 分布式锁、RabbitMQ 异步任务、JWT 鉴权、定时维护任务和线上部署。

## 在线访问

| 项目 | 地址 |
| --- | --- |
| 前端网站 | [https://www.fundtracking.online](https://www.fundtracking.online) |
| API 域名 | [https://api-fund.fundtracking.online](https://api-fund.fundtracking.online) |
| API 示例 | [https://api-fund.fundtracking.online/api/version](https://api-fund.fundtracking.online/api/version) |

## 核心功能

- 基金列表、基金详情和基金搜索
- 用户注册、登录、邮箱验证码和 JWT 鉴权
- 自选基金列表和提醒阈值管理
- 基金数据同步更新
- 基金数据异步更新
- Redis 缓存和缓存失效
- Redis 分布式锁，避免多个基金更新任务同时执行
- RabbitMQ 异步任务队列
- GitHub Actions 定时调用维护接口
- 阿里云 ECS + Nginx + systemd 部署

## 技术栈

| 分类 | 技术 |
| --- | --- |
| 前端 | Next.js / React / Vercel |
| 后端 | Go / Gin |
| 数据库 | MongoDB Atlas |
| 缓存与状态 | Redis |
| 消息队列 | RabbitMQ |
| 鉴权 | JWT / bcrypt |
| 部署 | 阿里云 ECS / Nginx / systemd / HTTPS |
| CI / 定时任务 | GitHub Actions |

## 当前架构

```text
浏览器
  -> Vercel 前端
  -> API 域名
  -> Nginx
  -> Go Gin API
  -> MongoDB Atlas / Redis / RabbitMQ
```

当前部署中：

- 前端部署在 Vercel。
- Go API 部署在阿里云 ECS。
- Nginx 负责 HTTPS 和 `/api` 反向代理。
- systemd 管理 Go 后端进程。
- MongoDB Atlas 保存线上数据。
- Redis 保存缓存、异步任务状态和更新锁。
- RabbitMQ 负责异步基金更新任务的排队和分发。

## 异步更新链路

当前异步更新使用一个 durable queue：`fund.update.tasks`。第一版 RabbitMQ consumer 与 Go API 在同一个进程中启动。

```text
POST /api/update/async
  -> Go API 校验 X-Update-Key
  -> 创建 task_id
  -> Redis 保存 pending 状态
  -> 发布 RabbitMQ 消息
  -> Worker 消费消息
  -> Redis 状态改为 running
  -> 获取 Redis update lock
  -> 执行基金抓取和 MongoDB upsert
  -> 删除相关 Redis 缓存
  -> Redis 状态改为 success / failed
  -> RabbitMQ ack
```

- Redis 保存 task 状态，可通过 `GET /api/update/tasks/:id` 查询。
- RabbitMQ 负责任务排队和分发。
- Worker 使用 manual ack。
- 第一版处理失败不自动 requeue，避免无限重试。
- Redis 分布式锁用于限制同一时间只有一个基金更新任务真正执行。
- RabbitMQ 消息发布失败时，Redis task 会被标记为 `failed`。

## 目录结构

```text
backend-go/
  main.go
  mq.go
  redis.go
  internal/
    update/
      handler.go
      service.go
      task.go
      lock.go
      worker.go
      message.go
```

| 文件 | 职责 |
| --- | --- |
| `backend-go/main.go` | 初始化 MongoDB、Redis、RabbitMQ，组装 update service、handler 和 worker |
| `backend-go/mq.go` | RabbitMQ 连接、durable queue 声明、消息发布和消费通道 |
| `backend-go/redis.go` | Redis 连接、普通缓存 key 和缓存失效 |
| `internal/update/handler.go` | HTTP handler 和 update 路由注册 |
| `internal/update/service.go` | 基金抓取、数据校验、MongoDB 写入和更新主流程 |
| `internal/update/task.go` | Redis task 状态读写 |
| `internal/update/lock.go` | Redis 分布式锁 |
| `internal/update/worker.go` | 消费 RabbitMQ 消息并执行基金更新 |
| `internal/update/message.go` | RabbitMQ update task 消息结构 |

## 主要接口

### 公开接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/funds` | 获取基金列表 |
| GET | `/api/fund/:code` | 获取基金详情 |
| GET | `/api/funds/:code/history` | 获取基金历史数据 |
| GET | `/api/search_proxy?query=` | 搜索基金 |
| GET | `/api/version` | 获取服务版本信息 |
| GET | `/api/health/mongo` | 检查 MongoDB 连接 |

### 鉴权接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/register` | 注册并发送邮箱验证码 |
| POST | `/api/auth/login` | 登录并返回 JWT |
| POST | `/api/auth/verify-email-code` | 校验邮箱验证码 |
| POST | `/api/auth/resend-email-code` | 重新发送邮箱验证码 |
| GET | `/api/auth/me` | 获取当前用户信息 |

### 自选基金接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/watchlist` | 获取当前用户的自选基金 |
| POST | `/api/watchlist` | 添加自选基金 |
| PUT | `/api/watchlist/:fundCode` | 更新提醒阈值 |
| DELETE | `/api/watchlist/:fundCode` | 删除自选基金 |

### 维护接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/update` | 同步更新基金数据 |
| POST | `/api/update/async` | 创建异步更新任务 |
| GET | `/api/update/tasks/:id` | 查询异步更新任务状态 |
| POST | `/api/funds/enrich` | 补充基金元数据 |
| POST | `/api/funds/performance` | 补充基金阶段表现字段 |
| GET | `/api/alerts/check` | 检查自选基金提醒 |
| POST | `/api/alerts/send` | 发送提醒邮件 |

维护接口需要通过 `X-Update-Key` 校验。`UPDATE_API_KEY` 只应保存在服务端环境变量或 Nginx 配置中，不应暴露给公开前端。

## 本地运行

### 1. 启动依赖

启动 Redis：

```bash
docker run --name fundtracking-redis -p 6379:6379 -d redis:7-alpine
```

启动 RabbitMQ：

```bash
docker run -d --name fundtracking-rabbitmq \
  --hostname fundtracking-rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  rabbitmq:4-management
```

可选：启动本地 MongoDB：

```bash
docker run -d --name fundtracking-mongo -p 27017:27017 mongo:7
```

如果本地无法连接 MongoDB Atlas，可以临时使用本地 MongoDB。本地 MongoDB 初始为空，触发基金更新任务后会 upsert 默认基金数据。

### 2. 启动 Go 后端

```bash
cd backend-go
go run .
```

### 3. 启动前端

```bash
cd client
npm install
npm run dev
```

## 环境变量

以下仅展示变量名称和本地示例，不包含真实线上密钥：

```text
MONGO_URI=
JWT_SECRET=
UPDATE_API_KEY=
REDIS_ADDR=127.0.0.1:6379
RABBITMQ_URL=amqp://guest:guest@127.0.0.1:5672/
RABBITMQ_UPDATE_QUEUE=fund.update.tasks
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
```

安全提醒：

- 真实密钥不要提交到仓库。
- 线上 RabbitMQ 不建议使用 `guest/guest`。
- `UPDATE_API_KEY` 只应放在服务端环境或 Nginx 配置中。
- `NEXT_PUBLIC_GO_API_URL` 是前端公开变量，不应放入任何密钥。

## RabbitMQ 验证步骤

1. 启动 Redis、RabbitMQ 和 MongoDB。
2. 启动 Go 后端。
3. 创建异步更新任务：

```bash
curl -X POST http://127.0.0.1:8081/api/update/async \
  -H "X-Update-Key: your_update_key"
```

4. 接口返回示例：

```json
{
  "status": "accepted",
  "task_id": "update_xxx"
}
```

5. 使用返回的 `task_id` 查询任务状态：

```bash
curl http://127.0.0.1:8081/api/update/tasks/update_xxx \
  -H "X-Update-Key: your_update_key"
```

6. 成功状态示例：

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

示例中的更新数量取决于当前默认基金、自选基金和外部数据源响应，实际结果可能不同。

7. 打开 RabbitMQ 管理后台：

```text
http://localhost:15672
```

本地默认登录：

```text
guest / guest
```

查看 `fund.update.tasks` 队列。任务处理完成后，`Ready` 和 `Unacked` 通常应回到 `0`。也可以观察消息发布和确认速率，验证任务确实经过 RabbitMQ 分发。

## 部署说明

- 前端部署在 Vercel。
- 后端部署在阿里云 ECS。
- Nginx 负责 HTTPS、API 域名和 `/api` 反向代理。
- systemd 管理 Go 后端进程。
- Redis 和 RabbitMQ 在服务器运行。
- MongoDB Atlas 作为线上数据库。
- GitHub Actions 定时调用受 `X-Update-Key` 保护的维护接口。

服务器安装并启动 RabbitMQ：

```bash
sudo apt install -y rabbitmq-server
sudo systemctl enable rabbitmq-server
sudo systemctl start rabbitmq-server
```

线上环境建议创建独立 RabbitMQ 用户，不使用默认 `guest/guest`：

```bash
sudo rabbitmqctl add_user fundtracking_mq 'strong_password'
sudo rabbitmqctl set_permissions -p / fundtracking_mq ".*" ".*" ".*"
```

示例中的密码是占位符，部署时应使用独立生成的安全密码，并通过服务器环境变量配置 `RABBITMQ_URL`。

常用线上检查：

```bash
curl -i https://api-fund.fundtracking.online/api/version
curl -i https://api-fund.fundtracking.online/api/funds
```

## 项目边界

- 项目只做基金信息展示、追踪和自选管理，不提供投资建议。
- 项目不提供真实交易、下单、支付能力。
- 项目主要用于 Go 后端实习项目展示和工程实践。
- 数据库 schema 默认不随意改动，优先保持现有 API 兼容。
- 真实密钥只保存在部署环境、GitHub Secrets 或服务器配置中，不提交到仓库。
- 当前 RabbitMQ consumer 与 Go API 在同一进程运行，尚未拆分独立 worker。

## 后续计划

- RabbitMQ publisher confirm
- 拆分独立 worker 进程
- 增加 DLQ / retry queue
- 提供更细粒度的更新任务进度
- 增加更完整的接口测试
- 继续整理其他模块的 internal 包结构
