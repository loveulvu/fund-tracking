# Fund Tracking

Fund Tracking 是一个基金跟踪系统，支持基金数据展示、已收录基金搜索、基金详情、用户注册登录、注册邮箱 6 位验证码、watchlist 关注列表、阈值提醒、未收录 6 位基金代码导入、基金元数据和阶段收益补全，以及工作日自动更新基金数据和发送提醒邮件。

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
  -> GET /api/update
  -> GET /api/alerts/check
  -> POST /api/alerts/send
  -> Render Go API
  -> MongoDB Atlas
  -> Resend
```

当前主链路是 Vercel 前端、Render Go API、MongoDB Atlas、GitHub Actions 和 Resend。Railway 只属于历史迁移阶段，不是当前主部署链路。

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

Deployment and automation:

- Vercel
- Render
- GitHub Actions
- Resend

## 核心功能

1. 基金列表展示
2. 基金详情展示
3. 已收录基金搜索
4. 用户注册和登录
5. 注册邮箱 6 位验证码
6. JWT 认证
7. Watchlist 添加、删除和阈值修改
8. 未收录 6 位基金代码手动导入
9. 基金元数据补全，包括类型、公司、经理、规模
10. 基金阶段收益补全，包括近 1 周、近 1 月、近 3 月、近 6 月、近 1 年、近 3 年
11. `/api/update` 自动更新基金基础行情
12. `/api/alerts/check` 检测 watchlist 阈值触发
13. `/api/alerts/send` 通过 Resend 发送提醒邮件
14. GitHub Actions 工作日定时执行 update/check/send

## Go 后端重构说明

项目从旧 Flask 后端逐步迁移到 Go 后端，当前核心线上接口已经由 Go 提供：

```text
GET    /api/version
GET    /api/health/mongo
GET    /api/funds
GET    /api/fund/{fundCode}
GET    /api/search_proxy?query=...
POST   /api/funds/import
POST   /api/funds/enrich
POST   /api/funds/performance
POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/verify-email-code
POST   /api/auth/resend-email-code
GET    /api/auth/me
GET    /api/watchlist
POST   /api/watchlist
PUT    /api/watchlist/{fundCode}
DELETE /api/watchlist/{fundCode}
GET    /api/update
GET    /api/alerts/check
POST   /api/alerts/send
```

迁移原则是保持前端 API 路径和 JSON 结构兼容，按模块增量替换旧后端，而不是一次性重写整套系统。

旧 Flask / Python 后端已经归档到 `archive/legacy-python/`，不再是当前线上主链路。

## 注册邮箱验证码

新注册用户需要完成邮箱 6 位数字验证码验证后才能登录。

验证码规则：

- 6 位数字验证码
- 10 分钟有效
- 最多错误 5 次
- 60 秒重发冷却
- 数据库只保存 HMAC-SHA256 hash，不保存验证码明文
- 老用户如果缺少 `emailVerified` 字段，登录时按已验证处理，避免历史账号被突然阻断

相关接口：

```text
POST /api/auth/register
POST /api/auth/verify-email-code
POST /api/auth/resend-email-code
POST /api/auth/login
```

验证码邮件使用 Resend 发送，但实现和基金提醒邮件逻辑隔离，避免注册邮件失败影响提醒链路。

## 数据更新机制

`/api/update` 用于更新基金基础行情数据，受 `UPDATE_API_KEY` 保护，普通前端用户不会直接调用。

更新范围：

```text
defaultFundCodes + distinct(watchlists.fundCode)
```

也就是先更新默认基金池，再更新所有用户 watchlist 中出现过的基金代码，并做全局去重。只有 6 位数字基金代码会进入更新队列，非法代码进入 `skipped_codes`。

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

## 未收录基金导入

前端 `/about` 页面只在用户输入完整 6 位数字基金代码、并且当前 MongoDB 未收录时，显示导入入口。

导入规则：

- 需要登录后调用 `POST /api/funds/import`
- 不对模糊搜索自动抓取外部数据
- 不自动加入用户 watchlist
- 基础行情抓取失败时不写入 MongoDB
- 元数据补全失败时保留基础行情，并返回部分成功信息
- 不补阶段收益字段，不把缺失值伪造成 0

已验证：

- `000001` 可以成功导入，基础行情和元数据可读
- `000002` 当前基础行情数据源不支持或返回失败，属于预期失败，不会写入坏数据

## 阶段收益补全

`POST /api/funds/performance` 用于补全阶段收益字段，受 `X-Update-Key` 保护。

当前补全字段：

```text
week_growth
month_growth
three_month_growth
six_month_growth
year_growth
three_year_growth
performanceUpdatedAt
```

字段映射来自 Eastmoney FundMNPeriodIncrease：

```text
Z  -> week_growth
Y  -> month_growth
3Y -> three_month_growth
6Y -> six_month_growth
1N -> year_growth
3N -> three_year_growth
```

写入规则：

- 只写有效数字字段和 `performanceUpdatedAt`
- 缺失、空字符串、`--`、解析失败都不写入
- 真实字符串 `"0"` 才会作为 0 写入
- 不 upsert 新基金
- 不修改基础行情字段
- 暂未接入 GitHub Actions，避免阶段收益补全失败影响稳定的 update/check/send 提醒链路

## 阈值提醒和邮件

提醒链路分三步：

1. `/api/update` 更新基金基础行情
2. `/api/alerts/check` 扫描 watchlist 阈值并写入 `alert_logs`
3. `/api/alerts/send` 读取待发送记录并通过 Resend 发送邮件

`alert_logs` 使用状态字段控制流转，核心状态包括：

```text
pending_email
email_ready
email_sent
email_failed
skipped_no_email
```

防重复规则：

```text
userId + fundCode + netValueDate + alertThreshold
```

同一用户、同一基金、同一净值日期、同一阈值条件不会重复发送。

已验证：

- 测试基金 `011839`
- 测试阈值 `0.01`
- `alert_logs.status = email_sent`
- Resend 返回 `providerMessageId`
- 目标邮箱已收到提醒邮件
- 测试后阈值已恢复
- 重复触发时不会重复发送

## GitHub Actions

默认分支 `main` 上的 workflow 在工作日 UTC 11:00 运行：

```text
cron: "0 11 * * 1-5"
```

UTC 11:00 对应中国和日本时间 20:00。

workflow 调用顺序：

```text
GET  /api/update
GET  /api/alerts/check
POST /api/alerts/send
```

每段响应都会上传 artifact：

```text
update-response.json
alerts-check-response.json
alerts-send-response.json
```

GitHub Actions 只保存调用后端所需的公开后端地址和更新密钥，不保存 Resend API key。

## 安全设计

1. `JWT_SECRET` 只在后端环境变量中。
2. `MONGO_URI` 只在后端环境变量中。
3. `UPDATE_API_KEY` 只在 Render 和 GitHub Actions Secrets 中。
4. `RESEND_API_KEY` 只在 Render 后端环境变量中。
5. `NEXT_PUBLIC_GO_API_URL` 只保存公开后端地址。
6. 不要把 `MONGO_URI`、`JWT_SECRET`、`UPDATE_API_KEY`、`RESEND_API_KEY` 放进任何 `NEXT_PUBLIC_*`。
7. `/api/update`、`/api/alerts/check`、`/api/alerts/send` 未携带正确 `X-Update-Key` 时不能执行。
8. watchlist 操作使用当前登录用户的 `userId + fundCode` 定位，避免跨用户误操作。

## 环境变量说明

Render 后端需要：

| 变量名 | 用途 |
| --- | --- |
| `MONGO_URI` | MongoDB Atlas 连接地址 |
| `JWT_SECRET` | JWT 签名密钥 |
| `UPDATE_API_KEY` | 保护后台 update/check/send 接口 |
| `RESEND_API_KEY` | Resend 邮件发送 |
| `ALERT_EMAIL_FROM` | 提醒邮件发件地址 |
| `ALERT_EMAIL_FROM_NAME` | 提醒邮件发件名称 |
| `APP_VERSION` | 后端版本标识 |

Vercel 前端需要：

| 变量名 | 用途 |
| --- | --- |
| `NEXT_PUBLIC_GO_API_URL` | 公开 Go 后端地址 |

GitHub Actions Secrets 需要：

| Secret 名 | 用途 |
| --- | --- |
| `BACKEND_BASE_URL` | Go 后端公开地址 |
| `UPDATE_API_KEY` | 与 Render 后端一致，用于调用后台任务接口 |

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

本地运行后端需要配置 `MONGO_URI`、`JWT_SECRET`、`UPDATE_API_KEY`。如果要本地验证真实邮件发送，还需要配置 Resend 相关环境变量。不要使用生产 secret 作为公开示例。

## 当前已验证线上状态

1. `/api/version` 正常。
2. `/api/health/mongo` 正常。
3. `/api/funds` 正常。
4. `/api/update` 受 `UPDATE_API_KEY` 保护。
5. `/api/funds/import` 可导入有效的未收录 6 位基金代码。
6. `/api/funds/enrich` 可补全基金类型、公司、经理和规模。
7. `/api/funds/performance` 可补全阶段收益字段，且缺失值不会写成 0。
8. 注册邮箱 6 位验证码前端真实收信验证通过。
9. GitHub Actions 可按 update/check/send 顺序执行。
10. Resend 邮件提醒链路已完成真实闭环。
11. 当前旧日期数据问题已修复。

## 当前限制和后续计划

当前不支持或未完成：

1. 阶段收益接口已实现，但暂未接入 GitHub Actions，仍需长期观察外部接口稳定性。
2. 持仓金额、资产配置和收益曲线未实现。
3. 不支持真实交易记录系统。
4. 不支持全市场模糊搜索自动外部导入。
5. Render Free Web Service 可能冷启动。
6. 旧 Flask / Python 代码已归档到 `archive/legacy-python/`，仅作为迁移参考。

后续计划：

1. 设计从 `go-backend-api-version` 合并到 `main` 的 PR 和部署验证策略。
2. 设计真实持仓和收益曲线的数据模型。
3. 评估是否将 `/api/funds/performance` 以非阻断方式接入 GitHub Actions。
4. 继续优化移动端 UI。
