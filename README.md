# Fund Tracking 基金信息追踪系统

## 项目简介

Fund Tracking 是一个基金信息追踪系统，用于展示基金列表、基金详情、搜索结果、自选基金和数据更新状态。

这个项目主要用于 Go 后端实习项目展示，不提供投资建议，也不提供交易能力。项目从原有 Python 后端逐步重构为 Go + Gin 后端，前端保持 Next.js / React 不变，重点练习前后端分离、MongoDB Atlas 数据访问、Redis 缓存与任务状态、JWT 鉴权、定时任务和一次接近真实后端项目流程的线上部署。

## 在线访问

| 项目 | 地址 |
| --- | --- |
| 前端网站 | `https://www.fundtracking.online` |
| API 域名 | `https://api-fund.fundtracking.online` |
| API 示例 | `https://api-fund.fundtracking.online/api/version` |

前端通过 `NEXT_PUBLIC_GO_API_URL` 指向线上 API：

```text
NEXT_PUBLIC_GO_API_URL=https://api-fund.fundtracking.online
```

## 核心功能

- 基金列表、基金详情和基金搜索。
- 用户注册、登录、邮箱验证码校验和 JWT 鉴权。
- 登录用户维护自选基金列表，并可设置提醒阈值。
- 后端支持同步更新和异步更新基金数据。
- GitHub Actions 定时调用维护类接口。
- Redis 保存缓存、更新锁和异步任务状态。

## 技术栈

| 分类 | 技术 |
| --- | --- |
| 前端 | Next.js、React、Vercel |
| 后端 | Go、Gin |
| 数据库 | MongoDB Atlas |
| 缓存与任务状态 | Redis |
| 鉴权 | JWT、bcrypt |
| 定时任务 | GitHub Actions |
| 部署 | 阿里云 ECS、Nginx、systemd、HTTPS、Vercel |

## 当前架构

```text
用户浏览器
  -> Vercel 前端 https://www.fundtracking.online
  -> API 域名 https://api-fund.fundtracking.online
  -> 阿里云 ECS 上的 Nginx
  -> Go Gin 后端 127.0.0.1:8081
  -> MongoDB Atlas / Redis
```

| 部分 | 当前配置 |
| --- | --- |
| 前端部署 | Vercel |
| 后端部署 | 阿里云 ECS |
| 后端服务 | Go + Gin |
| 后端监听 | 默认 `127.0.0.1:8081` |
| 反向代理 | Nginx 将 `/api/` 转发到 Go 后端 |
| 进程管理 | systemd 服务 `fund-tracking.service` |
| HTTPS | Nginx + Let's Encrypt |
| 数据存储 | MongoDB Atlas |
| 缓存与任务状态 | Redis |

## 后端设计

Go 后端位于 `backend-go/`，采用增量重构方式，不一次性推翻原有功能。当前方向是保持前端 API 路径和 JSON 结构兼容，在 Go + Gin 中逐步整理模块。

已整理的自选基金模块使用简单分层：

| 层级 | 职责 |
| --- | --- |
| handler | 处理 Gin 请求参数、JWT 用户信息、HTTP 状态码和 JSON 响应 |
| service | 处理参数校验、业务判断、缓存读取和缓存失效 |
| repository | 只负责 MongoDB 的查询、写入、删除和更新 |

后端还包含统一响应辅助函数、JWT 中间件、MongoDB 连接复用、Redis 工具函数，以及基金更新、基金导入、补充元数据、提醒检查等接口。

## Redis 使用

| 用途 | 说明 |
| --- | --- |
| 缓存 | 缓存部分读接口结果，减少重复查询 MongoDB |
| TTL | 缓存设置过期时间，避免长期返回旧数据 |
| Cache Aside | 读缓存未命中时查 MongoDB，再写回 Redis |
| 分布式锁 | 更新基金数据前加锁，避免重复触发更新任务 |
| 异步任务状态 | 异步更新创建任务后，将进度和结果写入 Redis 供前端轮询 |

以自选列表为例：查询时先读 Redis，未命中再查 MongoDB；新增、修改或删除自选基金后，让对应用户的缓存失效。

## 主要接口

### 公开基金接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/funds` | 获取基金列表 |
| GET | `/api/fund/:code` | 获取基金详情 |
| GET | `/api/search_proxy?query=` | 搜索基金 |
| GET | `/api/version` | 查看服务版本信息 |
| GET | `/api/health/mongo` | 检查 MongoDB 连接 |

### 用户与鉴权接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/register` | 注册并发送邮箱验证码 |
| POST | `/api/auth/login` | 登录并返回 JWT |
| POST | `/api/auth/verify-email-code` | 校验邮箱验证码 |
| POST | `/api/auth/resend-email-code` | 重发邮箱验证码 |
| GET | `/api/auth/me` | 获取当前用户信息 |

### 自选基金接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/watchlist` | 获取当前用户自选列表 |
| POST | `/api/watchlist` | 添加自选基金 |
| PUT | `/api/watchlist/:fundCode` | 更新提醒阈值 |
| DELETE | `/api/watchlist/:fundCode` | 删除自选基金 |

### 维护类接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/update` | 同步更新基金数据 |
| POST | `/api/update/async` | 创建异步更新任务 |
| GET | `/api/update/tasks/:id` | 查询异步更新任务状态 |
| POST | `/api/funds/enrich` | 补充基金元数据 |
| POST | `/api/funds/performance` | 补充阶段表现字段 |
| GET | `/api/alerts/check` | 检查自选提醒 |
| POST | `/api/alerts/send` | 发送提醒邮件 |

维护类接口需要 `X-Update-Key`。这个 key 只放在服务端环境变量或 Nginx 配置中，不放入前端公开变量。

## 异步更新流程

浏览器触发异步更新时，不直接暴露 `UPDATE_API_KEY`。当前做法是在 Nginx 中提供浏览器可访问的入口，由 Nginx 在服务端注入 `X-Update-Key` 后转发到 Go 后端。

```text
创建任务：
浏览器
  -> /api/update/async-client
  -> Nginx 注入 X-Update-Key
  -> Go 后端 /api/update/async
  -> Redis 保存任务状态

查询任务：
浏览器
  -> /api/update/tasks-client/{task_id}
  -> Nginx 注入 X-Update-Key
  -> Go 后端 /api/update/tasks/{task_id}
  -> Redis 返回任务状态
```

GitHub Actions 中的定时任务会使用仓库 Secrets 中的 `BACKEND_BASE_URL` 和 `UPDATE_API_KEY` 调用维护接口。

## 本地运行

后端需要 MongoDB Atlas 连接串、JWT 密钥、更新接口密钥等环境变量，可参考 `.env.example` 和 `backend-go/.env.example`。

启动 Redis：

```bash
docker run --name fundtracking-redis -p 6379:6379 -d redis:7-alpine
```

启动 Go 后端：

```bash
cd backend-go
go run .
```

检查接口：

```bash
curl http://127.0.0.1:8081/api/version
curl http://127.0.0.1:8081/api/health/mongo
```

启动前端：

```bash
cd client
npm install
npm run dev
```

本地前端可使用：

```text
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
```

## 部署说明

- 前端部署在 Vercel，项目目录为 `client/`。
- 后端部署在阿里云 ECS，当前使用 Linux amd64 Go 二进制文件运行。
- Nginx 负责 HTTPS、API 域名和 `/api/` 反向代理。
- systemd 管理 Go 后端进程，服务名为 `fund-tracking.service`。
- MongoDB Atlas 作为主要数据库，Redis 用于缓存、锁和异步任务状态。
- GitHub Actions 提供工作日定时维护任务，也支持手动触发。
- `backend-go/Dockerfile` 保留用于容器化尝试或本地实验，当前线上后端不是通过 Docker 运行。

常用线上检查：

```bash
curl -i https://api-fund.fundtracking.online/api/version
curl -i https://api-fund.fundtracking.online/api/funds
```

## 项目边界

- 项目只做基金信息展示、追踪和自选管理，不提供投资建议。
- 项目不提供真实交易、下单或支付能力。
- 当前重点是 Go 后端实习项目展示和逐步重构过程，不追求复杂微服务架构。
- 数据库 schema 默认不随意改动，优先保持前端 API 兼容。
- 真实密钥只应保存在部署环境、GitHub Secrets 或服务器配置中，不应提交到仓库。
