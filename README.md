# Fund Tracking

Fund Tracking 是一个基金跟踪项目，当前处于 Go 后端重构阶段。

当前目标：

- 保持现有前端 API 行为尽量稳定。
- 用 Go 后端逐步替换原 Flask 基金数据接口。
- 保留 Flask 后端中的 auth / watchlist / email 等旧功能。
- 保留 GitHub Actions 定时刷新基金数据的架构。

## Project Scope

当前重构范围：

- Go 后端负责基金读取、详情、搜索、基础数据更新。
- Flask 后端暂时继续负责用户认证、watchlist、邮件相关功能。
- 前端保持 Next.js，不做大规模重写。
- MongoDB 继续作为主数据库。

## Tech Stack

Frontend:

- Next.js 16
- React 19
- CSS Modules

Legacy Backend:

- Flask
- Flask-CORS
- PyMongo
- PyJWT
- Requests

Go Backend:

- Go
- net/http
- MongoDB Go Driver

Database:

- MongoDB

Email:

- Resend API

Automation:

- GitHub Actions

## Backend Structure

Legacy Flask backend:

```text
backend/
  app/
    __init__.py
    config.py
    extensions.py
    version.py
    routes/
      auth.py
      funds.py
      watchlist.py
      health.py
    services/
      fund_fetcher.py
      fund_service.py
      auth_service.py
      email_service.py
    models/
      user.py
      fund.py
    utils/
      security.py
      response.py
  run.py
```

Go backend:

```text
backend-go/
  main.go
  go.mod
  go.sum
```

说明：

- `backend/app/__init__.py` 是 Flask app factory 入口。
- 根目录 `app.py` 是旧部署兼容入口。
- `backend/run.py` 也可以启动 Flask app。
- `backend-go/main.go` 是当前 Go 后端入口。

## Environment Variables

Backend:

- `MONGO_URI`：MongoDB 连接地址，必填。
- `JWT_SECRET`：JWT 密钥，建议配置。
- `EMAIL_SENDER`：邮件发送地址，注册 / 重发验证码功能需要。
- `EMAIL_PASSWORD`：邮件密钥，注册 / 重发验证码功能需要。
- `UPDATE_API_KEY`：基金更新接口鉴权 key，可选。
- `AUTO_REFRESH_INTERVAL_SECONDS`：旧 Flask 自动刷新间隔，可选，默认 `180`。
- `APP_VERSION`：应用版本，可选，默认 `dev`。
- `APP_BUILT_AT`：构建时间，可选。
- `GIT_COMMIT` / `RAILWAY_GIT_COMMIT_SHA` / `SOURCE_VERSION`：提交版本信息，可选。
- `PORT`：Flask 后端端口，可选，默认 `8080`。

Frontend:

- `NEXT_PUBLIC_API_URL`：旧 Flask API 地址。
- `NEXT_PUBLIC_GO_API_URL`：Go 后端 API 地址，本地默认 `http://127.0.0.1:8081`。

GitHub Actions Secrets:

- `BACKEND_BASE_URL`：公网后端地址，用于 GitHub Actions 定时触发 `/api/update`。
- `UPDATE_API_KEY`：如果 Go 后端配置了更新接口鉴权，Actions 需要同步配置该 secret。
- `MONGO_URI`：旧 monitor 任务需要。
- `RESEND_API_KEY` / `MAIL_PASSWORD`：邮件提醒任务需要。
- `SENDER_EMAIL` / `MAIL_SENDER`：邮件发送地址。

注意：不要把真实的 `MONGO_URI`、`UPDATE_API_KEY`、邮件密钥写进 README 或提交到仓库。

## Local Run

### 1. 安装依赖

```bash
pip install -r requirements.txt
cd client && npm install
```

### 2. 配置环境变量

复制根目录环境变量示例：

```bash
cp .env.example .env
```

复制前端环境变量示例：

```bash
cp client/.env.example client/.env.local
```

Windows PowerShell 下可以手动复制文件。

### 3. 启动 Go 后端

```bash
cd backend-go
go run main.go
```

Go 后端本地地址：

```text
http://127.0.0.1:8081
```

### 4. 如需旧功能，启动 Flask 后端

如果需要 auth / watchlist / email 等旧接口，可以启动 Flask：

```bash
python app.py
```

或者：

```bash
python backend/run.py
```

### 5. 启动前端

```bash
cd client
npm run dev
```

## Go Backend Endpoints

当前 Go 后端已实现：

```text
GET /api/health/mongo
GET /api/funds
GET /api/fund/<fund_code>
GET /api/search_proxy?query=...
GET /api/funds/search?query=...
GET /api/update
```

### GET /api/health/mongo

检查 Go 后端是否能连接 MongoDB。

成功返回示例：

```json
{
  "status": "ok",
  "message": "MongoDB connected"
}
```

### GET /api/funds

返回 MongoDB 中的基金列表。

调用链：

```text
/api/funds
-> fundsHandler
-> loadFundsFromMongoDB
-> findFundsByFilter
-> MongoDB Find
-> JSON response
```

### GET /api/fund/<fund_code>

按基金代码查询单只基金详情。

示例：

```text
GET /api/fund/008540
```

返回规则：

- 找到：返回基金 JSON。
- 找不到：返回 `404 Fund not found`。
- 系统错误：返回 `500`。

### GET /api/search_proxy?query=...

按基金代码或基金名称搜索基金。

示例：

```text
GET /api/search_proxy?query=008
```

### GET /api/funds/search?query=...

搜索兼容路由，内部复用 `/api/search_proxy` 的处理逻辑。

### GET /api/update

Go 后端已实现基金基础数据更新接口。

该接口会：

```text
遍历默认基金池
-> 请求外部基金接口
-> 解析 JSONP 数据
-> 转换为内部 Fund 字段
-> 使用 MongoDB upsert 写入 fund_data
-> 返回更新结果
```

返回示例：

```json
{
  "status": "success",
  "updated": 10,
  "failed": [],
  "total": 10
}
```

当前 Go 更新范围：

- `fund_code`
- `fund_name`
- `net_value`
- `day_growth`
- `net_value_date`
- `update_time`
- `is_seed`

当前版本暂时不会刷新以下完整资料字段：

- 基金公司
- 基金经理
- 基金规模
- 近 1 周收益
- 近 1 月收益
- 近 3 月收益
- 近 6 月收益
- 近 1 年收益
- 近 3 年收益

如果配置了 `UPDATE_API_KEY`，请求必须携带以下任意一种鉴权方式：

```text
X-Update-Key: <UPDATE_API_KEY>
```

或：

```text
/api/update?key=<UPDATE_API_KEY>
```

如果没有配置 `UPDATE_API_KEY`，本地开发环境下该接口会直接放行。

## Frontend API Routing

当前 `client/src/lib/api.js` 中：

已切到 Go 后端的接口：

```text
getFunds
getFund
searchFunds
updateFunds
```

仍走旧 Flask API 的接口：

```text
getWatchlist
addToWatchlist
removeFromWatchlist
updateWatchlistThreshold
login
register
verifyEmail
resendVerification
```

当前前端接口层中：

```text
apiUrl(path)
```

用于拼接旧 Flask API 地址。

```text
goApiUrl(path)
```

用于拼接 Go 后端 API 地址。

本地开发时，`goApiUrl('/api/update')` 通常会指向：

```text
http://127.0.0.1:8081/api/update
```

## GitHub Actions Scheduled Updates

项目中存在 GitHub Actions 定时任务。

主要相关 workflow：

```text
.github/workflows/trading_update.yml
.github/workflows/cron_monitor.yml
.github/workflows/monitor.yml
```

### trading_update.yml

这是当前主要的基金数据定时更新链路。

它会在交易时间内定时执行：

```text
BACKEND_BASE_URL + /api/update
```

实际逻辑：

```text
GitHub Actions schedule
-> 读取 BACKEND_BASE_URL
-> 拼接 BACKEND_BASE_URL/api/update
-> curl 请求更新接口
-> 如果配置 UPDATE_API_KEY，则通过 X-Update-Key 请求头传给后端
-> 保存 update-response.json artifact
```

本地开发时 Go 后端地址是：

```text
http://127.0.0.1:8081
```

但是 GitHub Actions 不能访问开发者本机的 `127.0.0.1`。

因此，后续 Go 后端部署后，`BACKEND_BASE_URL` 必须设置为 Go 后端的公网地址。

示例：

```text
BACKEND_BASE_URL=https://your-go-backend-domain.com
```

### cron_monitor.yml / monitor.yml

这两个 workflow 会运行：

```text
python monitor_task.py
```

`monitor_task.py` 当前主要用于：

```text
读取 watchlist
-> 获取关注基金当前涨跌幅
-> 判断是否超过用户设置的阈值
-> 触发邮件提醒
```

它更接近 watchlist 邮件提醒任务，不是当前 Go 后端的主基金数据更新链路。

当前主要基金数据更新链路是：

```text
trading_update.yml -> GET /api/update
```

## API Compatibility

前端相关接口兼容列表：

```text
GET /api/funds
GET /api/fund/<fund_code>
GET /api/search_proxy?query=...
GET /api/funds/search?query=...
GET /api/update
GET /api/watchlist
POST /api/watchlist
DELETE /api/watchlist/<fund_code>
PUT /api/watchlist/<fund_code>
POST /api/auth/login
POST /api/auth/register
POST /api/auth/verify
POST /api/auth/resend
GET /api/version
GET /api/health
GET /health
```

## Deployment

当前旧部署入口：

```text
web: python app.py
```

也就是说，目前 `Procfile` 仍然面向 Flask 后端。

Go 后端后续如果部署到 Railway / Render / Fly.io 等平台，需要单独配置 Go 服务启动命令。

本地 Go 后端地址：

```text
http://127.0.0.1:8081
```

不能直接用于线上环境。

线上环境应配置：

```text
NEXT_PUBLIC_GO_API_URL=https://your-go-backend-domain.com
BACKEND_BASE_URL=https://your-go-backend-domain.com
```

## Known Runtime Risks

- 如果 `MONGO_URI` 缺失或错误，Go 后端无法读取或更新基金数据。
- 如果外部基金接口超时，`/api/update` 可能出现部分基金更新失败。
- 当前 Go `/api/update` 只更新基础字段，不能覆盖完整基金资料字段。
- 如果错误地把整个 Fund 结构 `$set` 进 MongoDB，可能把已有完整字段覆盖成空值或 0。
- 如果 `UPDATE_API_KEY` 配置不一致，GitHub Actions 调用 `/api/update` 会返回 401。
- GitHub Actions 无法访问本机 `127.0.0.1`，必须使用公网 Go 后端地址。
- 旧 Flask 和新 Go 在迁移期会并存，接口归属需要保持清楚。
- 邮件变量缺失时，注册 / 重发验证码 / watchlist 邮件提醒可能失败。
- 外部基金 API 可能返回 JSONP，不是标准 JSON，需要先清洗再解析。