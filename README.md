# Fund Tracking

Fund Tracking 是一个基金跟踪系统，支持基金数据展示、搜索、用户注册登录、watchlist 关注列表、阈值管理，以及工作日自动更新基金净值数据。

## 在线地址

前端：

https://www.fundtracking.online

Go 后端：

https://fund-tracking-go-api.onrender.com

## 当前架构

```text
Browser
  -> Vercel Next.js Frontend
  -> Render Go API
  -> MongoDB Atlas

GitHub Actions
  -> /api/update
  -> Render Go API
  -> MongoDB Atlas
```

当前主链路是 Vercel 前端、Render Go API 和 MongoDB Atlas。Railway 只属于历史迁移阶段，不是当前主部署链路。

## 技术栈

Frontend:

- Next.js
- React
- CSS Modules

Backend:

- Go
- net/http
- MongoDB Go Driver
- JWT
- bcrypt

Database:

- MongoDB Atlas

Deployment:

- Vercel
- Render
- GitHub Actions

## 核心功能

1. 基金列表展示
2. 基金搜索
3. 基金详情
4. 用户注册
5. 用户登录
6. JWT 认证
7. Watchlist 添加、删除、阈值修改
8. 基金数据更新接口 `/api/update`
9. GitHub Actions 工作日定时更新

## Go 后端重构说明

项目正在从旧 Flask 后端迁移到 Go 后端。当前核心线上接口已经迁移到 Go：

```text
GET    /api/version
GET    /api/health/mongo
GET    /api/funds
GET    /api/fund/{fundCode}
GET    /api/search_proxy?query=...
POST   /api/auth/register
POST   /api/auth/login
GET    /api/auth/me
GET    /api/watchlist
POST   /api/watchlist
PUT    /api/watchlist/{fundCode}
DELETE /api/watchlist/{fundCode}
GET    /api/update
```

Go 后端保持现有前端 API 路径兼容，避免在迁移过程中破坏前端主流程。

## 数据更新机制

`/api/update` 用于更新基金净值数据，当前机制如下：

1. `/api/update` 受 `UPDATE_API_KEY` 保护。
2. 普通前端用户不会直接调用 `/api/update`。
3. GitHub Actions 在工作日 UTC 11:00 自动调用一次。
4. UTC 11:00 对应中国和日本时间 20:00。
5. 更新范围是 `defaultFundCodes + distinct(watchlists.fundCode)`。
6. 非法基金代码会在更新前进入 `skipped_codes`。
7. 已进入更新队列但抓取、校验或写入失败的基金会进入 `failed_codes`。

`/api/update` 返回字段包括：

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

## 安全设计

1. `JWT_SECRET` 只在后端环境变量中配置。
2. `UPDATE_API_KEY` 只在 Render 环境变量和 GitHub Secrets 中配置。
3. `MONGO_URI` 只在 Render 环境变量中配置。
4. `NEXT_PUBLIC_GO_API_URL` 只保存公开后端地址。
5. 不要把 `MONGO_URI`、`JWT_SECRET`、`UPDATE_API_KEY` 放进任何 `NEXT_PUBLIC_*` 变量。
6. `/api/update` 未携带正确 key 时返回 `401`。
7. watchlist 操作使用 `userId + fundCode` 定位记录，避免跨用户误操作。

## 环境变量说明

Render 后端需要：

| 变量名 | 用途 |
| --- | --- |
| `MONGO_URI` | MongoDB Atlas 连接地址 |
| `JWT_SECRET` | JWT 签名密钥 |
| `UPDATE_API_KEY` | 保护 `/api/update` 的调用密钥 |
| `APP_VERSION` | 后端版本标识 |

Vercel 前端需要：

| 变量名 | 用途 |
| --- | --- |
| `NEXT_PUBLIC_GO_API_URL` | 公开 Go 后端地址，例如 `https://fund-tracking-go-api.onrender.com` |

GitHub Actions Secrets 需要：

| Secret 名 | 用途 |
| --- | --- |
| `BACKEND_BASE_URL` | Go 后端公开地址，例如 `https://fund-tracking-go-api.onrender.com` |
| `UPDATE_API_KEY` | 与 Render 后端环境变量保持一致 |

不要把真实 secret 写入代码、README 或前端公开环境变量。

## 本地开发

Backend:

```bash
cd backend-go
go run .
```

Frontend:

```bash
cd client
npm install
npm run dev
```

本地运行后端需要配置 `MONGO_URI`、`JWT_SECRET`、`UPDATE_API_KEY`。开发环境可以使用本地或测试数据库，不需要使用生产 secret。

## 当前线上状态

当前线上状态已验证：

1. `/api/version` 正常。
2. `/api/health/mongo` 正常。
3. `/api/funds` 正常。
4. `/api/update` 受 key 保护。
5. GitHub Actions 手动触发成功。
6. 当前数据已无 `2026-04-01` 旧日期。

最近一次验收中，`/api/funds` 返回 12 条基金数据，包含默认基金和 watchlist 中额外出现的基金代码。

## 项目限制和后续计划

当前限制：

1. Render Free Web Service 可能冷启动。
2. 旧 Flask 代码仍存在，需要后续归档或清理。
3. 前端 UI 还需要进一步统一。
4. 邮件模块暂未恢复。

后续计划：

1. 统一前端 UI。
2. 清理旧 Flask 和 Railway 迁移痕迹。
3. 增加项目讲述文档。
4. 评估邮件提醒模块。
5. 可选：后端自定义域名 `api.fundtracking.online`。
