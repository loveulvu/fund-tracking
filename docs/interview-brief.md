# Fund Tracking 面试讲述稿

## 一、30 秒项目介绍

Fund Tracking 是一个已上线的基金跟踪系统。前端使用 Next.js 部署在 Vercel，后端从旧 Flask 逐步迁移到 Go `net/http`，部署在 Render，数据存储使用 MongoDB Atlas。

系统支持基金列表和详情、已收录基金搜索、注册登录、JWT 鉴权、watchlist、阈值提醒、未收录 6 位基金代码导入、基金元数据补全，以及 GitHub Actions 工作日自动执行基金更新、提醒检测和 Resend 邮件发送。

## 二、3 分钟项目讲述稿

### 1. 项目背景

这个项目最开始是一个 Flask 后端加 Next.js 前端的基金跟踪系统。随着功能增加，后端逐渐包含基金数据、用户认证、关注列表、提醒阈值、外部数据抓取和邮件提醒等逻辑。

后来我把核心线上接口增量迁移到 Go，一方面提升接口结构的可维护性，另一方面练习 Go 在真实 Web API、MongoDB 读写、JWT 鉴权和后台任务中的使用。

### 2. 技术栈

前端使用 Next.js、React 和 CSS Modules，部署在 Vercel。

后端主链路使用 Go，基于标准库 `net/http` 实现路由和 handler，MongoDB 使用官方 Go Driver，认证使用 JWT 和 bcrypt。

数据库使用 MongoDB Atlas。自动化任务使用 GitHub Actions。提醒邮件使用 Resend。当前线上架构是 Vercel 前端、Render Go 后端、MongoDB Atlas、GitHub Actions 和 Resend。

### 3. 前端架构

前端保留 Next.js Pages Router。主要页面包括总览、基金列表、基金详情、登录、注册和账户页。

API 封装集中在 `client/src/lib/api.js`，通过 `NEXT_PUBLIC_GO_API_URL` 请求 Go 后端。登录后 token 保存在浏览器本地，用于调用 watchlist、导入基金和 `/api/auth/me` 等受保护接口。

UI 已从旧的黑色页面统一成浅色金融 dashboard 风格。基金列表页支持搜索已收录基金，如果用户输入完整 6 位基金代码且数据库未收录，会显示导入入口。

### 4. Go 后端重构

Go 后端当前负责核心线上接口，包括：

- `GET /api/version`
- `GET /api/health/mongo`
- `GET /api/funds`
- `GET /api/fund/{fundCode}`
- `GET /api/search_proxy?query=...`
- `POST /api/funds/import`
- `POST /api/funds/enrich`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/watchlist`
- `POST /api/watchlist`
- `PUT /api/watchlist/{fundCode}`
- `DELETE /api/watchlist/{fundCode}`
- `GET /api/update`
- `GET /api/alerts/check`
- `POST /api/alerts/send`

迁移原则是保持前端 API 兼容，不一次性推翻旧系统，而是按模块逐步替换。

### 5. MongoDB 数据设计

主要 collection 包括：

- `fund_data`：保存基金代码、名称、净值、日涨跌、净值日期、更新时间、基金类型、基金公司、基金经理、基金规模等。
- `users`：保存用户邮箱和密码哈希。
- `watchlists`：保存用户关注基金，包含 `userId`、`fundCode`、`fundName`、`alertThreshold` 等。
- `alert_logs`：保存提醒触发和邮件发送状态，用于防重复提醒。

基金基础行情更新使用 `UpdateOne + $set + upsert`，以 `fund_code` 匹配。元数据补全只 `$set` 有效非空字段，不会把已有元数据覆盖为空。

### 6. JWT 登录认证

注册时使用 bcrypt 保存密码哈希。登录成功后签发 JWT，前端后续请求在 `Authorization: Bearer <token>` 中携带 token。

Go 后端通过 auth middleware 解析 JWT，得到当前用户 ID。watchlist、基金导入、`/api/auth/me` 等接口都依赖这个认证上下文。

### 7. Watchlist 设计

watchlist 的关键约束是使用 `userId + fundCode` 定位记录。这样同一只基金可以被不同用户关注，但用户只能修改或删除自己的关注记录，避免跨用户误操作。

watchlist 中的 `alertThreshold` 用于阈值提醒检测。账户页会把 watchlist 和基金行情按 `fundCode` 合并展示。如果没有行情数据，页面显示“暂无行情数据”，而不是显示误导性的 0。

### 8. `/api/update` 数据更新机制

`/api/update` 从外部基金数据源抓取基础行情并写入 MongoDB。接口受 `UPDATE_API_KEY` 保护，普通前端用户不会直接调用。

当前更新范围是：

```text
defaultFundCodes + distinct(watchlists.fundCode)
```

只允许 6 位数字基金代码进入更新队列。接口返回 `target_codes`、`updated_codes`、`failed_codes`、`skipped_codes`，方便排查到底哪些基金被尝试更新、哪些成功、哪些失败。

如果外部基础行情抓取失败、基金代码不匹配、净值解析失败或校验失败，就不写入 MongoDB，避免污染数据。

### 9. GitHub Actions 定时更新

默认分支 `main` 上的 GitHub Actions 在工作日 UTC 11:00 运行，对应中国和日本时间 20:00。

workflow 顺序调用：

```text
GET  /api/update
GET  /api/alerts/check
POST /api/alerts/send
```

每段响应都会上传 artifact：

- `update-response.json`
- `alerts-check-response.json`
- `alerts-send-response.json`

GitHub Actions 只使用 `BACKEND_BASE_URL` 和 `UPDATE_API_KEY`。Resend API key 保留在 Render 后端环境变量中，不进入 GitHub Actions。

### 10. 遇到的问题和解决方案

一个典型问题是线上部分基金长期停留在旧日期。排查后发现 `/api/funds` 读取 `fund_data` 全量数据，但旧版 `/api/update` 只更新固定默认基金池。后来我把更新范围扩展为 `defaultFundCodes + distinct(watchlists.fundCode)`，并增强返回结构，旧日期问题得到解决。

另一个问题是部分阶段收益字段缺失时被显示成 `0.00%`。根因是 Go 结构体使用 `float64`，MongoDB 缺失字段解码后变成 0。修复方式是把可缺失阶段收益字段改成可空值，前端区分 null 和真实 0。缺失显示“暂无数据”，真实 0 才显示 `0.00%`。

邮件提醒方面，我把提醒检测和邮件发送拆成两个接口。`/api/alerts/check` 只负责扫描阈值和写入 `alert_logs`，`/api/alerts/send` 负责发送待处理邮件。`alert_logs` 通过状态机和去重键避免重复发送。

## 三、核心技术亮点

1. **Flask 后端迁移到 Go 后端**  
   使用增量迁移策略，保持前端 API 兼容，逐步把核心接口切到 Go。

2. **Go `net/http` 路由和 handler**  
   使用标准库实现 REST API，包括基金查询、认证、watchlist、导入、更新和提醒。

3. **MongoDB 查询、upsert 和 projection**
   查询时控制返回字段，写入时使用 `UpdateOne + $set + upsert`，用 `fund_code` 定位基金数据。

4. **JWT 鉴权中间件**  
   登录后签发 JWT，受保护接口通过 middleware 解析用户身份。

5. **`userId + fundCode` 防止跨用户误操作**  
   watchlist 修改和删除都绑定当前用户和基金代码。

6. **`UPDATE_API_KEY` 保护后台任务接口**
   update/check/send 不暴露给普通前端用户，统一通过 `X-Update-Key` 调用。

7. **GitHub Actions 定时任务**
   默认分支定时执行 update/check/send，并上传 artifact 便于验收。

8. **`alert_logs` 防重复状态机**
   使用 `pending_email`、`email_ready`、`email_sent`、`email_failed`、`skipped_no_email` 管理提醒发送状态。

9. **Resend 邮件提醒闭环**
   真实验证过阈值触发、日志写入、邮件发送、provider message id 和防重复逻辑。

10. **缺失字段不伪造为 0**
    阶段收益缺失返回 null，前端显示“暂无数据”，避免误导用户。

11. **未收录基金手动导入**
    仅完整 6 位基金代码允许导入，基础行情失败不写库，元数据只写有效非空字段。

## 四、可讲的难点和解决方案

### 1. 为什么首页数据旧到 2026-04-01？

`/api/funds` 读取的是 `fund_data` 全量数据，但旧版 `/api/update` 只更新 `defaultFundCodes`。如果 MongoDB 里有默认池外的旧数据，它不会被覆盖。

解决方案是把更新范围扩展为：

```text
defaultFundCodes + distinct(watchlists.fundCode)
```

### 2. 为什么不能把 `/api/update` 改成更新全量 `fund_data`？

全量更新成本更高，也可能包含历史测试数据或不再需要的数据。当前只更新默认池和用户关注过的基金，更符合真实页面和用户需求。

### 3. 为什么不能把 `UPDATE_API_KEY` 或 `RESEND_API_KEY` 放进 `NEXT_PUBLIC_*`？

`NEXT_PUBLIC_*` 会被打包到前端，任何访问网站的人都能看到。更新密钥和邮件密钥都必须留在后端环境变量或 GitHub Secrets 中。

### 4. 为什么 GitHub Actions schedule 要同步到 main？

GitHub Actions 的 `schedule` 只在默认分支生效。如果 workflow 只在功能分支修改，定时任务不会按新逻辑运行。

### 5. 为什么 Resend key 不放 GitHub Actions？

GitHub Actions 只调用后端接口，不直接发邮件。真正发邮件的是 Render 后端，所以 Resend key 应该只在 Render 后端环境变量中。

### 6. 为什么 `alert_logs` 要做防重复？

同一用户、同一基金、同一净值日期、同一阈值条件如果每天重复运行任务，可能反复发同一封提醒。通过 `userId + fundCode + netValueDate + alertThreshold` 去重，可以避免重复提醒。

### 7. 为什么收益曲线暂时不实现？

真实收益曲线需要持仓份额、买卖记录、历史净值快照或持仓权重。当前系统只有基金净值和关注列表，不足以计算真实组合收益。

### 8. 为什么阶段收益缺失时显示“暂无数据”？

缺失和真实 0 是两个不同含义。真实 0 可以显示 `0.00%`，但缺失不能被伪造成 0，否则会误导用户。

### 9. 为什么搜索不到时不自动抓取？

模糊搜索可能触发大量外部请求和写库。当前只允许完整 6 位基金代码由登录用户手动导入，降低误触发和数据污染风险。

### 10. 外部基金数据源失败时怎么处理？

基础行情失败时不写库，直接返回失败。元数据失败时不回滚基础行情，只返回部分成功。这样可以保证核心行情数据的正确性。

## 五、面试问答

### 1. 你为什么要把 Flask 后端迁移到 Go？

为了提升接口结构的可维护性和稳定性，也为了实践 Go 在真实 Web API 中的使用。迁移采用增量方式，保持前端 API 兼容，降低风险。

### 2. Go 后端为什么使用 `net/http`？

项目规模还不大，标准库足够实现路由、handler 和 middleware。使用标准库可以减少依赖，也更清楚地理解 HTTP 请求处理流程。

### 3. MongoDB 的基金数据如何写入？

基础行情用 `fund_code` 作为 filter，使用 `UpdateOne + $set + upsert` 写入。元数据补全只 `$set` 有效非空字段，不覆盖成空值。

### 4. 为什么阶段收益字段要用可空值？

因为 MongoDB 缺失字段如果解码成 `float64` 会变成 0，前端无法区分“真实 0”和“缺失”。使用可空值后，缺失可以返回 null。

### 5. JWT 登录流程是什么？

注册时保存 bcrypt 密码哈希。登录时校验密码，成功后签发 JWT。前端请求受保护接口时携带 `Authorization: Bearer <token>`，后端 middleware 解析用户身份。

### 6. watchlist 如何避免跨用户误操作？

查询、修改、删除都使用当前用户的 `userId` 加 `fundCode` 定位，而不是只按基金代码操作。

### 7. `/api/update` 为什么需要 `UPDATE_API_KEY`？

它会触发外部抓取和数据库写入，不能向普通前端用户开放。只有后台任务或受控调用方可以带 `X-Update-Key` 执行。

### 8. GitHub Actions 做了什么？

工作日晚上自动调用 `/api/update`、`/api/alerts/check` 和 `/api/alerts/send`，并上传三段 JSON artifact 便于排查。

### 9. 提醒邮件如何防重复？

`alert_logs` 以 `userId + fundCode + netValueDate + alertThreshold` 去重。已经 `email_sent` 的记录不会重复发送。

### 10. 邮件发送失败如何处理？

单条失败不会中断整体任务。失败记录会进入 `email_failed`，保存 `lastError`、`lastAttemptAt` 和 `retryCount`，方便后续重试策略。

### 11. 为什么 `/api/alerts/check` 和 `/api/alerts/send` 要拆开？

检测和发送职责不同。拆开后可以分别观察触发结果和发送结果，也能避免外部邮件服务失败影响阈值检测。

### 12. 未收录基金如何导入？

用户输入完整 6 位基金代码且 MongoDB 未收录时，登录后可调用 `POST /api/funds/import`。接口先抓基础行情，校验成功才写库，再尝试补全元数据。

### 13. 如果基金代码数据源不支持怎么办？

基础行情源失败时不写库。例如 `000002` 当前数据源不支持或返回失败，这属于预期失败，不是代码 bug。

### 14. 当前系统为什么不支持全市场模糊搜索？

全市场模糊搜索需要更完整的数据源和限流策略。当前只查询 MongoDB 已收录基金，未收录导入只支持完整 6 位代码。

### 15. 当前最大不足是什么？

持仓和收益曲线还没有真实数据支撑，阶段收益字段的数据源也没有完全统一。后续需要先设计数据模型和可靠数据源。

## 六、项目不足和后续计划

当前不足：

1. Render Free 可能冷启动。
2. 旧 Flask 代码仍需后续归档或清理。
3. 注册验证邮件尚未恢复。
4. 阶段收益字段仍未统一接入可靠数据源。
5. 持仓金额、收益曲线和资产配置未实现。
6. 不支持真实交易记录系统。
7. 移动端 UI 仍可继续优化。

后续计划：

1. 清理旧 Flask 迁移痕迹。
2. 评估阶段收益字段数据源。
3. 设计持仓和收益曲线的数据模型。
4. 评估注册验证邮件恢复方案。
5. 优化移动端基金列表和账户页。

## 七、简历项目描述

- 将基金跟踪系统核心后端从 Flask 增量迁移到 Go，基于 `net/http`、MongoDB Go Driver、JWT 和 bcrypt 实现基金查询、认证、watchlist、导入、更新和提醒接口，并保持前端 API 兼容。
- 设计基金数据更新和导入链路，支持 `defaultFundCodes + distinct(watchlists.fundCode)` 自动更新、6 位基金代码手动导入、元数据补全，并在外部数据源失败时避免写入无效数据。
- 搭建 Vercel 前端、Render Go 后端、MongoDB Atlas、GitHub Actions 和 Resend 的线上链路，实现工作日自动 update/check/send，支持阈值提醒邮件、防重复发送和 artifact 可观测性。
