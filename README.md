# Fund Tracking

## 项目定位
Fund Tracking 是一个全栈基金追踪系统，面向个人投资者提供基金行情查看、搜索、账号登录和自选管理能力。  
当前仓库目标是稳定线上 API 行为，支持前后端独立部署，并保持前端对既有接口的兼容。

## 技术栈
- 前端: Next.js 16, React 19, CSS Modules
- 后端: Flask, Flask-CORS, PyMongo, PyJWT, Requests
- 数据源: 东财相关公开接口
- 数据库: MongoDB
- 邮件: Resend API（用于注册验证码）
- 部署: Railway（后端）, Node.js 运行环境（前端）

## 核心功能
- 基金列表与单基金详情查询
- 基金搜索（代码/名称）
- 用户注册、邮箱验证码验证、登录（JWT）
- 自选列表管理（新增、删除、阈值更新）
- 默认种子基金自动初始化与刷新
- 健康检查与版本信息接口

## 目录结构
```text
.
|- app.py                         # 兼容入口，内部调用 backend.app.create_app()
|- backend/
|  |- run.py                      # 模块化后端入口
|  `- app/
|     |- __init__.py              # Flask app factory
|     |- config.py
|     |- extensions.py
|     |- version.py
|     |- routes/
|     |  |- auth.py
|     |  |- funds.py
|     |  |- watchlist.py
|     |  `- health.py
|     |- services/
|     |  |- fund_fetcher.py
|     |  |- fund_service.py
|     |  |- auth_service.py
|     |  `- email_service.py
|     |- models/
|     |  |- user.py
|     |  `- fund.py
|     `- utils/
|        |- security.py
|        `- response.py
|- client/
|  |- package.json
|  |- .env.example
|  `- src/
|     |- pages/
|     |- components/
|     `- lib/
|- monitor_task.py
|- cron_update.py
|- requirements.txt
`- Procfile
```

## 环境变量
后端（必填/选填）:
- `MONGO_URI`（必填）: MongoDB 连接串
- `JWT_SECRET`（建议必填）: JWT 签名密钥
- `EMAIL_SENDER`（选填）: Resend 发件人地址
- `EMAIL_PASSWORD`（选填）: Resend API Key
- `UPDATE_API_KEY`（选填）: `/api/update*` 接口保护密钥
- `AUTO_REFRESH_INTERVAL_SECONDS`（选填，默认 `180`）: 自动刷新间隔
- `APP_VERSION`（选填，默认 `dev`）: 后端版本号
- `APP_BUILT_AT`（选填）: 构建时间标识
- `GIT_COMMIT` / `RAILWAY_GIT_COMMIT_SHA` / `SOURCE_VERSION`（选填）: 提交哈希
- `PORT`（选填，默认 `8080`）: 后端监听端口

前端:
- `NEXT_PUBLIC_API_URL`（必填）: 后端 API 根地址，例如 `http://localhost:8080`

## 本地启动
1. 安装依赖
```bash
pip install -r requirements.txt
cd client && npm install
```

2. 配置环境变量
- 后端在项目根目录配置运行环境（如系统环境变量或 `.env` 注入）
- 前端创建 `client/.env.local`:
```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

3. 启动后端（任选其一）
```bash
python app.py
```
```bash
python backend/run.py
```

4. 启动前端
```bash
cd client
npm run dev
```

5. 访问地址
- 前端: `http://localhost:3000`
- 后端健康检查: `http://localhost:8080/health`
- 后端版本接口: `http://localhost:8080/api/version`

## 部署说明
- 当前 `Procfile` 使用 `web: python app.py`，可直接用于 Railway。
- `app.py` 已保留兼容入口，内部调用模块化后端，不影响现有部署命令。
- 生产环境建议至少配置:
  - `MONGO_URI`
  - `JWT_SECRET`
  - `NEXT_PUBLIC_API_URL`（前端）
- 若启用更新保护，请配置 `UPDATE_API_KEY`，并通过 `X-Update-Key` 请求头或 `?key=` 访问 `/api/update` 与 `/api/update_seeds`。

## 已知问题
- 第三方基金接口偶发超时或字段缺失，后端会回退默认值并记录日志。
- 邮箱验证码功能依赖 Resend 配置，若未配置会导致注册验证码发送失败。
- 目前缺少自动化测试流水线，接口兼容主要依赖人工回归。
- 历史数据中存在少量遗留字符编码异常（仅影响部分日志/兜底名称，不影响接口路径与核心流程）。

## 现有 API 路径（保持兼容）
- `GET /`
- `GET /health`
- `GET /api/health`
- `GET /api/version`
- `GET /api/funds`
- `GET /api/fund/<fund_code>`
- `GET /api/search_proxy?query=...`
- `GET /api/funds/search?query=...`
- `GET /api/update`
- `GET /api/update_seeds`
- `GET /api/watchlist`
- `POST /api/watchlist`
- `DELETE /api/watchlist/<fund_code>`
- `PUT /api/watchlist/<fund_code>`
- `POST /api/auth/register`
- `POST /api/auth/verify`
- `POST /api/auth/resend`
- `POST /api/auth/login`

