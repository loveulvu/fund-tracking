# Fund Tracking 面试讲述稿

## 一、30 秒项目介绍

Fund Tracking 是一个已上线的基金跟踪系统，前端使用 Next.js 部署在 Vercel，后端从旧 Flask 逐步迁移到 Go，并部署在 Render，数据存储使用 MongoDB Atlas。

系统支持基金列表展示、基金搜索、用户注册登录、JWT 认证、watchlist 关注列表、关注基金阈值管理，以及通过 GitHub Actions 在工作日自动触发 `/api/update` 更新基金净值数据。当前线上主链路已经切到 Go 后端，基金数据旧日期问题也通过扩展更新范围解决。

## 二、3 分钟项目讲述稿

### 1. 项目背景

这个项目最开始是一个 Flask 后端加 Next.js 前端的基金跟踪系统。随着功能增加，后端接口逐渐包含基金数据、用户认证、关注列表、邮件提醒等逻辑。后续我希望把核心 API 迁移到 Go，一方面提升后端结构的可维护性，另一方面也练习用 Go 实现真实的 Web API、MongoDB 读写和 JWT 鉴权。

### 2. 技术栈

前端使用 Next.js、React 和 CSS Modules，部署在 Vercel。

后端当前主链路使用 Go，主要基于标准库 `net/http` 实现路由和 handler，MongoDB 使用官方 Go Driver，认证使用 JWT 和 bcrypt。

数据库使用 MongoDB Atlas。自动化更新使用 GitHub Actions。线上部署是 Vercel 前端、Render Go 后端、MongoDB Atlas 数据库。

### 3. 前端架构

前端仍保留 Next.js Pages Router 结构，页面主要包括首页总览、基金列表、登录/注册、账户页和基金详情页。

前端 API 封装集中在 `client/src/lib/api.js`，通过 `NEXT_PUBLIC_GO_API_URL` 请求 Go 后端。用户登录后 token 保存在浏览器本地，用于调用需要认证的 watchlist 和 `/api/auth/me` 等接口。

最近一轮 UI 收口把旧黑色纹理页面统一为浅色金融 dashboard 风格，导航收敛为「总览 / 基金列表 / 登录」或登录后的「总览 / 基金列表 / 账户」。

### 4. Go 后端重构

Go 后端负责当前核心线上接口，包括：

- `GET /api/version`
- `GET /api/health/mongo`
- `GET /api/funds`
- `GET /api/fund/{fundCode}`
- `GET /api/search_proxy?query=...`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/watchlist`
- `POST /api/watchlist`
- `PUT /api/watchlist/{fundCode}`
- `DELETE /api/watchlist/{fundCode}`
- `GET /api/update`

迁移时的原则是保持前端 API 路径和 JSON 结构兼容，不一次性推翻旧系统，而是按模块逐步替换。

### 5. MongoDB 数据设计

主要 collection 包括基金数据和关注列表：

- `fund_data`：存储基金代码、基金名称、净值、日涨跌、净值日期、更新时间等字段。
- `watchlists`：按用户记录关注的基金，包含 `userId`、`fundCode`、`fundName`、`alertThreshold`、`addedAt` 等字段。

基金查询使用 MongoDB projection 去掉 `_id`，前端直接消费 API JSON。更新基金数据时使用 `UpdateOne + $set + upsert`，以 `fund_code` 作为匹配条件。

### 6. JWT 登录认证

注册时后端使用 bcrypt 保存密码哈希。登录成功后签发 JWT，前端后续请求在 `Authorization: Bearer <token>` 中携带 token。

Go 后端通过 auth middleware 解析 token，提取当前用户 ID，并把用户身份传给需要认证的接口，例如 `/api/watchlist` 和 `/api/auth/me`。

### 7. Watchlist 设计

watchlist 的核心约束是使用 `userId + fundCode` 定位记录。这样同一个基金可以被不同用户关注，但用户只能操作自己的关注记录，避免跨用户误删或误改。

前端支持添加关注、取消关注和修改提醒阈值。账户页会把 watchlist 中的 `fundCode` 和 `/api/funds` 的行情数据按基金代码合并展示。如果某只关注基金没有行情数据，页面显示「暂无行情数据」，而不是让用户看到裸露的空值。

### 8. `/api/update` 数据更新机制

`/api/update` 用于从外部基金接口抓取净值数据并写入 MongoDB。这个接口受 `UPDATE_API_KEY` 保护，普通前端用户不会直接调用。

当前更新范围是：

```text
defaultFundCodes + distinct(watchlists.fundCode)
```

也就是说，系统会先更新默认基金池，再补充所有用户关注列表中出现过的基金代码，并做全局去重。只有 6 位数字基金代码会进入更新队列，非法代码进入 `skipped_codes`。

接口返回会包含：

- `status`
- `updated`
- `failed`
- `total`
- `duration_ms`
- `target_codes`
- `updated_codes`
- `failed_codes`
- `skipped_codes`

这样可以明确知道本次尝试更新了哪些基金、哪些成功、哪些失败、哪些被跳过。

### 9. GitHub Actions 定时更新

GitHub Actions 在默认分支 `main` 上配置了工作日定时任务：

```text
cron: "0 11 * * 1-5"
```

这个时间是 UTC 11:00，对应中国和日本时间 20:00。workflow 从 GitHub Secrets 中读取后端公开地址和更新 key，调用：

```text
${BACKEND_BASE_URL%/}/api/update
```

如果配置了更新 key，就通过 `X-Update-Key` 请求头传给后端。workflow 会保存 `update-response.json` 并上传 artifact，方便检查 `target_codes`、`updated_codes` 和 `failed_codes`。

### 10. 遇到的问题和解决方案

一个典型问题是线上部分基金数据长期停留在 `2026-04-01`。排查后发现 `/api/funds` 读取的是 `fund_data` 全量数据，但 `/api/update` 原来只更新 `defaultFundCodes` 中的 10 只基金。页面上如果出现默认基金池之外的旧 seed 数据，就不会被覆盖。

解决方案是把更新范围扩展为 `defaultFundCodes + distinct(watchlists.fundCode)`，同时增强 `/api/update` 返回结构，明确展示 `target_codes`、`updated_codes`、`failed_codes`、`skipped_codes`。手动触发 GitHub Actions 后，线上 `/api/funds` 返回 12 条数据，旧日期数量已经为 0。

## 三、核心技术亮点

1. **Flask 后端迁移到 Go 后端**  
   采用增量迁移方式，保持前端 API 兼容，不一次性重写整个系统。

2. **Go `net/http` 路由和 handler**  
   用标准库实现 REST API，包括基金查询、认证、watchlist CRUD 和更新接口。

3. **MongoDB 查询、upsert、projection**  
   查询时使用 projection 控制返回字段；更新基金数据时使用 `UpdateOne + $set + upsert`，以 `fund_code` 定位数据。

4. **JWT 鉴权中间件**  
   登录后签发 JWT，受保护接口通过 middleware 解析用户身份。

5. **`userId + fundCode` 防止跨用户误操作**  
   watchlist 的查询、更新、删除都绑定当前用户和基金代码。

6. **`UPDATE_API_KEY` 保护更新接口**  
   `/api/update` 不对普通前端开放，未携带正确 key 返回 401。

7. **GitHub Actions 工作日定时调用**  
   使用 schedule 和 workflow_dispatch 自动或手动触发数据更新，并上传响应 artifact 方便验收。

8. **Vercel + Render + MongoDB Atlas 架构**  
   前端、后端、数据库独立部署，环境变量和 secrets 分层管理。

## 四、可讲的难点和解决方案

### 1. 为什么首页数据旧到 2026-04-01？

因为 `/api/funds` 读取 `fund_data` collection 全量数据，而旧版 `/api/update` 只更新固定的 `defaultFundCodes`。如果 MongoDB 中存在默认基金之外的旧 seed 数据，它们不会被更新接口覆盖。

### 2. 为什么 `/api/update` 原来只更新 `defaultFundCodes` 不够？

用户关注列表可能包含默认基金池之外的基金。如果更新接口只处理默认基金池，那么用户关注的新基金即使展示在页面里，也可能长期停留在旧净值日期。

### 3. 为什么改成 `defaultFundCodes + distinct(watchlists.fundCode)`？

默认基金池保证首页基础数据稳定；watchlist 中出现过的基金代表真实用户关心的数据。把两者合并并去重，可以覆盖当前系统最需要更新的数据范围，同时避免直接更新 `fund_data` 全量带来的成本和风险。

### 4. 为什么不能把 `UPDATE_API_KEY` 放进 `NEXT_PUBLIC_*`？

`NEXT_PUBLIC_*` 会被打包进前端代码，任何访问网站的人都能看到。`UPDATE_API_KEY` 是保护后台更新接口的密钥，只能放在 Render 后端环境变量和 GitHub Secrets 中。

### 5. 为什么 GitHub Actions schedule 要同步到 `main`？

GitHub Actions 的 `schedule` 只会从默认分支运行。如果只在功能分支修改 workflow，即使代码正确，定时任务也不会按新的 cron 生效。所以定时更新 workflow 需要同步到默认分支 `main`。

### 6. 为什么 Render Free 会有冷启动？

Render Free Web Service 在空闲一段时间后可能休眠。下一次请求会触发实例启动，所以首个请求可能较慢。这是免费部署层的运行特性，不是接口逻辑错误。

### 7. 为什么收益曲线暂时不实现？

真实组合收益曲线需要历史净值快照、用户持仓份额、买入卖出记录或至少持仓权重。当前系统只存基金净值和关注列表，没有足够数据计算真实组合收益。如果现在做曲线，只能伪造数据，不适合展示为真实功能。

## 五、面试问答

### 1. 你为什么要把 Flask 后端迁移到 Go？

主要是为了提升后端接口的可维护性和运行稳定性，同时练习 Go 在真实 Web API 中的使用。迁移不是一次性推翻，而是按接口逐步替换，保持前端 API 兼容，降低风险。

### 2. Go 后端路由是怎么组织的？

当前 Go 后端使用 `net/http` 注册路由，不同路径对应不同 handler。例如基金列表、基金详情、搜索、认证和 watchlist 都有独立 handler。需要登录的接口包一层 auth middleware。

### 3. 为什么没有一开始就引入 Gin 或 Echo？

项目规模还比较小，标准库 `net/http` 已经能满足路由和 handler 需求。先用标准库能减少依赖，也更容易理解 HTTP 请求处理、middleware 和 JSON 响应的底层流程。

### 4. MongoDB 中基金数据怎么更新？

更新时以 `fund_code` 作为 filter，使用 `UpdateOne` 和 `$set` 写入核心字段，并开启 upsert。这样如果基金已存在就更新，如果不存在就插入。

### 5. 为什么查询基金列表时要做 projection？

MongoDB 默认会返回 `_id`。前端不需要这个字段，而且 `_id` 直接序列化可能带来额外格式问题。所以查询时用 projection 去掉 `_id`，让 API 返回更干净。

### 6. JWT 登录流程是什么？

用户登录时后端校验邮箱和密码，密码通过 bcrypt 与数据库里的 hash 比对。验证通过后签发 JWT。前端把 token 放在 `Authorization: Bearer <token>` 请求头中，后端 middleware 解析 token 得到用户身份。

### 7. watchlist 怎么避免用户之间互相影响？

watchlist 操作都绑定当前登录用户。删除或修改阈值时，不只按 `fundCode` 查，而是使用当前 `userId + fundCode` 组合定位记录，避免一个用户操作到另一个用户的数据。

### 8. `/api/update` 为什么要加 `UPDATE_API_KEY`？

更新接口会触发外部抓取和数据库写入，不应该让任何前端用户随意调用。加 `UPDATE_API_KEY` 后，只有 Render 环境变量和 GitHub Actions Secrets 中持有 key 的自动任务能调用。

### 9. GitHub Actions 是怎么触发数据更新的？

workflow 在默认分支 `main` 上配置 schedule，工作日 UTC 11:00 自动运行。它读取 `BACKEND_BASE_URL` 和 `UPDATE_API_KEY`，请求 `${BACKEND_BASE_URL%/}/api/update`，并保存响应 JSON 作为 artifact。

### 10. 为什么数据更新范围不是直接更新 MongoDB 里所有基金？

全量更新成本更高，也可能包含历史测试数据或不再需要的数据。当前选择更新默认基金池和所有用户关注过的基金，覆盖真实页面和用户需求，同时控制更新范围。

### 11. 外部基金接口失败时怎么处理？

如果抓取失败、返回格式异常、基金代码不匹配或净值解析失败，该基金不会写入 MongoDB，而是进入 `failed_codes`，详细错误进入 `failed`。这样可以避免把坏数据写入数据库。

### 12. 前端如何知道后端地址？

前端通过 `NEXT_PUBLIC_GO_API_URL` 配置公开 Go 后端地址。这个变量只保存公开 URL，不包含任何密钥。真正的后端密钥只放在后端环境变量或 GitHub Secrets。

### 13. 为什么 GitHub Actions workflow 要同步到 `main`？

因为 schedule 只在默认分支生效。功能分支上的 schedule 不会定时跑，所以定时更新 workflow 必须存在于默认分支。

### 14. 这个项目的主要安全边界是什么？

主要有三层：JWT 保护用户级接口；`userId + fundCode` 限制 watchlist 操作范围；`UPDATE_API_KEY` 保护数据更新接口。并且密钥不放入 `NEXT_PUBLIC_*`。

### 15. 你在项目中做了哪些取舍？

没有实现收益曲线，因为缺少真实历史净值和持仓数据；没有立即删除旧 Flask 代码，因为迁移要保持可回退和可对照；UI 先做轻量统一，没有引入新 UI 框架，避免重构成本过高。

## 六、项目不足和后续计划

1. **Render Free 会冷启动**  
   线上后端首个请求可能变慢，后续可以考虑付费实例或其他部署方案。

2. **旧 Flask 代码仍需清理**  
   当前核心线上 API 已迁移到 Go，但仓库中仍保留旧 Flask 代码，后续需要归档或删除迁移痕迹。

3. **邮件模块暂未恢复**  
   旧邮件提醒和邮箱验证模块没有作为当前主链路恢复，后续需要重新设计安全和发送流程。

4. **收益曲线暂未实现**  
   需要历史净值快照、用户持仓份额和收益计算逻辑，当前系统还没有这些数据结构。

5. **移动端 UI 仍可继续优化**  
   当前主要完成了桌面 dashboard 风格统一，移动端表格和账户页还可以继续打磨。

## 七、简历项目描述

- 将基金跟踪系统核心后端从 Flask 增量迁移到 Go，基于 `net/http`、MongoDB Go Driver、JWT 和 bcrypt 实现基金查询、认证、watchlist CRUD 和数据更新接口，并保持前端 API 兼容。
- 设计并修复基金数据自动更新链路，将 `/api/update` 更新范围从固定默认基金池扩展为 `defaultFundCodes + distinct(watchlists.fundCode)`，通过 upsert 写入 MongoDB，并返回 `target_codes`、`updated_codes`、`failed_codes` 等可观测字段。
- 完成 Vercel 前端、Render Go 后端、MongoDB Atlas 和 GitHub Actions 的线上部署与自动化更新配置，使用 GitHub Actions 在工作日定时调用受 `UPDATE_API_KEY` 保护的更新接口。
