# Fund Tracking

Fund Tracking 是一个基金跟踪系统，用于展示和管理基金数据。项目支持基金列表、基金详情、搜索、自选列表、登录注册，以及异步更新基金数据。

这个项目的定位是 Go 后端实习项目展示，重点在于前后端分离、Go Gin 后端重构、缓存与异步任务、以及一次真实线上部署链路的排查与落地。它不是金融交易系统，也不提供投资建议或交易能力。

## Current Online Architecture

当前线上链路：

- Frontend: Vercel
- Website: `https://www.fundtracking.online`
- API domain: `https://api-fund.fundtracking.online`
- Backend: Alibaba Cloud ECS 上的 Go Gin 后端
- Backend binary: `/opt/fund-tracking/backend-go/fund-tracking-go-api`
- systemd service: `fund-tracking.service`
- systemd env file: `/etc/fund-tracking.env`
- Backend listen address: `127.0.0.1:8081` by default
- Nginx config: `/www/server/panel/vhost/nginx/fund-tracking-api.conf`
- Nginx proxy: `/api/ -> http://127.0.0.1:8081/api/`
- HTTPS certificate: Let's Encrypt

```text
User Browser
  -> Vercel Frontend (https://www.fundtracking.online)
  -> API Domain (https://api-fund.fundtracking.online)
  -> Alibaba Cloud Nginx
  -> Go Gin Backend (127.0.0.1:8081)
  -> MongoDB Atlas / Redis
```

The frontend reads `NEXT_PUBLIC_GO_API_URL` to decide which API host to call. Current production value:

```text
NEXT_PUBLIC_GO_API_URL=https://api-fund.fundtracking.online
```

## Async Update Flow

The async update button no longer uses a Vercel API Route as a proxy.

Reason: Vercel Serverless Functions had unstable HTTPS connectivity to the Alibaba Cloud API domain (`ECONNRESET`), while HTTP requests were blocked by Alibaba Cloud ICP filing checks. The current solution keeps the browser on HTTPS and lets Nginx inject the private update key server-side.

Task creation:

```text
Browser
  -> https://api-fund.fundtracking.online/api/update/async-client
  -> Nginx injects X-Update-Key
  -> http://127.0.0.1:8081/api/update/async
  -> Go backend creates async update task
  -> Redis stores task status
```

Task polling:

```text
Browser
  -> https://api-fund.fundtracking.online/api/update/tasks-client/{task_id}
  -> Nginx injects X-Update-Key
  -> http://127.0.0.1:8081/api/update/tasks/{task_id}
  -> Go backend reads Redis task status
```

Verified production logs include:

```text
POST /api/update/async status=202
GET /api/update/tasks/{task_id} status=200
GET /api/funds status=200
```

`/api/update/async-client` and `/api/update/tasks-client/{task_id}` are Nginx browser-facing entries. They are not native Go business routes. The native Go routes are `/api/update/async` and `/api/update/tasks/:id`.

## Tech Stack

| Layer | Stack |
| --- | --- |
| Frontend | Next.js, React, Vercel |
| Backend | Go, Gin |
| Database | MongoDB Atlas |
| Cache / Lock / Task Status | Redis |
| Deployment | Vercel, Alibaba Cloud ECS, Nginx, systemd, Let's Encrypt |
| Auth | JWT, bcrypt |
| Scheduled Task | GitHub Actions workflow exists in `.github/workflows/trading_update.yml` |

The GitHub Actions workflow is scheduled at UTC 11:00 on weekdays and can also be triggered manually. It uses `BACKEND_BASE_URL` and `UPDATE_API_KEY` secrets and uploads response artifacts. The current workflow file should be kept aligned with the deployed Gin route methods before relying on it for production maintenance.

## Backend Capabilities

- REST API for fund data, auth, watchlist, update tasks, alerts, and metadata enrichment
- Gin route registration and middleware
- JWT authentication for user APIs
- MongoDB queries and updates for fund data, users, and watchlists
- Redis cache for selected read paths
- Redis lock to avoid overlapping fund update jobs
- Redis task status storage for async updates
- Nginx reverse proxy with server-side injection of update key for browser-triggered async update routes
- systemd service management for the Go backend process

## Project Structure

```text
.
├── backend-go/                 # Go Gin backend
│   ├── main.go                 # startup, Mongo init, Gin routes
│   ├── update.go               # sync and async fund update handlers
│   ├── auth.go                 # register, login, JWT, email verification
│   ├── watchlist.go            # watchlist APIs
│   ├── alerts.go               # alert check and email send APIs
│   ├── import.go               # import a fund by code
│   ├── enrich.go               # fund metadata enrichment
│   ├── performance.go          # period performance enrichment
│   ├── redis.go                # Redis cache, lock, task status
│   └── Dockerfile
├── client/                     # Next.js frontend
│   └── src/
├── .github/workflows/
│   └── trading_update.yml      # scheduled maintenance workflow
├── docs/
└── README.md
```

## Main APIs

Public fund data:

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/funds` | Fund list |
| GET | `/api/fund/:code` | Fund detail |
| GET | `/api/search_proxy?query=` | Fund search |
| GET | `/api/version` | Service version metadata |
| GET | `/api/health/mongo` | MongoDB health check |

Auth:

| Method | Path | Description |
| --- | --- | --- |
| POST | `/api/auth/register` | Register and send verification code |
| POST | `/api/auth/login` | Login and return JWT |
| POST | `/api/auth/verify-email-code` | Verify email code |
| POST | `/api/auth/resend-email-code` | Resend email verification code |
| GET | `/api/auth/me` | Current user info |

Watchlist:

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/watchlist` | List current user's watchlist |
| POST | `/api/watchlist` | Add fund to watchlist |
| PUT | `/api/watchlist/:fundCode` | Update alert threshold |
| DELETE | `/api/watchlist/:fundCode` | Remove watchlist item |

Protected maintenance APIs:

| Method | Path | Description |
| --- | --- | --- |
| POST | `/api/update` | Synchronous fund data update |
| POST | `/api/update/async` | Create async update task |
| GET | `/api/update/tasks/:id` | Read async update task status |
| POST | `/api/funds/enrich` | Enrich fund metadata |
| POST | `/api/funds/performance` | Enrich period performance fields |
| GET | `/api/alerts/check` | Check watchlist alert thresholds |
| POST | `/api/alerts/send` | Send pending alert emails |

Notes:

- `/api/update` and `/api/update/async` require `X-Update-Key`.
- The frontend must not expose `UPDATE_API_KEY`.
- Browser-triggered async update uses Nginx entries `/api/update/async-client` and `/api/update/tasks-client/{task_id}`. Nginx injects `X-Update-Key` before proxying to the Go backend.

## Environment Variables

Backend:

| Variable | Description |
| --- | --- |
| `MONGO_URI` | MongoDB Atlas connection string |
| `JWT_SECRET` | JWT signing secret |
| `UPDATE_API_KEY` | Private key for protected update/maintenance APIs |
| `REDIS_ADDR` | Redis address, for example `127.0.0.1:6379` |
| `APP_VERSION` | Version label returned by `/api/version` |
| `PORT` | Optional. If unset, the backend listens on `127.0.0.1:8081` |

The backend also supports `REDIS_URL` when using a managed Redis URL. Do not commit real secrets.

Frontend / Vercel:

| Variable | Description |
| --- | --- |
| `NEXT_PUBLIC_GO_API_URL` | Public API base URL. Production: `https://api-fund.fundtracking.online` |

Do not put `UPDATE_API_KEY` in any `NEXT_PUBLIC_*` variable. Async update no longer depends on a Vercel API Route proxy.

## Local Development

Start Redis locally if needed:

```bash
docker run --name fundtracking-redis -p 6379:6379 -d redis:7-alpine
```

Run the Go backend:

```bash
cd backend-go
go run .
```

Health checks:

```bash
curl http://127.0.0.1:8081/api/version
curl http://127.0.0.1:8081/api/health/mongo
```

Run the frontend:

```bash
cd client
npm install
npm run dev
```

Local frontend API base example:

```text
NEXT_PUBLIC_GO_API_URL=http://127.0.0.1:8081
```

## Deployment Notes

### Frontend

The frontend is deployed on Vercel with `client` as the project directory.

Production environment:

```text
NEXT_PUBLIC_GO_API_URL=https://api-fund.fundtracking.online
```

After changing frontend code, deploy Vercel Production again.

### Backend

Current production deployment uses a Linux amd64 binary managed by systemd, not Docker. `backend-go/Dockerfile` is kept in the repository for container-based local or experimental builds.

Build a Linux amd64 binary from the local repository:

```bash
cd backend-go
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o fund-tracking-go-api-linux-amd64 .
```

Upload it to ECS, then back up the old binary before replacing it:

```bash
sudo cp /opt/fund-tracking/backend-go/fund-tracking-go-api \
  /opt/fund-tracking/backend-go/fund-tracking-go-api.bak.$(date +%Y%m%d%H%M%S)

sudo install -m 755 /tmp/fund-tracking-go-api.new \
  /opt/fund-tracking/backend-go/fund-tracking-go-api
```

Restart and inspect the service:

```bash
sudo systemctl restart fund-tracking
sudo systemctl status fund-tracking --no-pager
sudo journalctl -u fund-tracking -n 100 --no-pager
```

Validate Nginx after editing its server config:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

Basic production checks:

```bash
curl -i https://api-fund.fundtracking.online/api/version
curl -i https://api-fund.fundtracking.online/api/funds
```

For protected update routes, keep `UPDATE_API_KEY` only in server-side files such as `/etc/fund-tracking.env` and Nginx config. Do not commit it.

## Nginx Async Update Entries

The API domain's 443 server block contains browser-facing entries for async update. These entries inject `X-Update-Key` on the server side and proxy to the private Go routes on `127.0.0.1:8081`.

Do not add these entries to the port 80 server block.

Example shape, with the real key kept only on the server:

```nginx
location = /api/update/async-client {
    proxy_pass http://127.0.0.1:8081/api/update/async;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header X-Update-Key "<server-side UPDATE_API_KEY>";
    proxy_connect_timeout 30s;
    proxy_send_timeout 30s;
    proxy_read_timeout 30s;
}

location ~ ^/api/update/tasks-client/([^/]+)$ {
    proxy_pass http://127.0.0.1:8081/api/update/tasks/$1;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header X-Update-Key "<server-side UPDATE_API_KEY>";
    proxy_connect_timeout 30s;
    proxy_send_timeout 30s;
    proxy_read_timeout 30s;
}
```

## GitHub Actions Scheduled Task

The repository includes `.github/workflows/trading_update.yml`.

Current workflow behavior:

- Trigger: weekday schedule at `0 11 * * 1-5` UTC and manual `workflow_dispatch`
- Secrets: `BACKEND_BASE_URL`, `UPDATE_API_KEY`
- Calls update, alert check, and alert send endpoints
- Uploads response JSON files as workflow artifacts for 7 days

Before using it for production maintenance, verify that the workflow's HTTP methods match the currently deployed Gin routes.

## Deployment Notes / Troubleshooting

Some deployment issues encountered during the current setup:

- The frontend and backend are deployed separately: Vercel for frontend, Alibaba Cloud ECS for the Go API.
- `api.fundtracking.online` previously pointed to an older Railway path. The current API domain is `api-fund.fundtracking.online`.
- An older binary on Alibaba Cloud returned `405` for `POST /api/update/async`; deploying the Gin refactor binary fixed the route.
- Vercel Serverless Functions had `ECONNRESET` when forwarding update requests to the Alibaba Cloud HTTPS API domain.
- HTTP requests to the API domain were blocked by Alibaba Cloud ICP filing checks, so protected browser update routes must stay HTTPS-only.
- The current async update solution uses Nginx `async-client` and `tasks-client` entries to inject `X-Update-Key` server-side.

Useful checks:

```bash
sudo systemctl status fund-tracking --no-pager
sudo journalctl -u fund-tracking -n 100 --no-pager
sudo nginx -t
curl -i https://api-fund.fundtracking.online/api/version
curl -i https://api-fund.fundtracking.online/api/funds
```

## Project Scope

- This project displays and manages fund tracking data. It is not a trading platform.
- It should not be described as a financial-grade or enterprise-grade system.
- It does not use Kubernetes or a microservice architecture.
- MongoDB Atlas is the primary data store. Redis is used for cache, update coordination, and short-lived task status.
- Secrets should stay in deployment environments and server-only config files, not in the frontend or repository.
