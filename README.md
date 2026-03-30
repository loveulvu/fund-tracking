# 基金追踪系统 (Fund Tracking System)

一个完整的基金数据追踪和分析系统，支持实时抓取基金数据、用户注册登录、基金标记和涨幅提醒功能。

## 项目结构

```
├── client/            # 前端项目（Next.js）
│   ├── public/        # 静态资源
│   ├── src/           # 源代码
│   │   ├── components/ # 组件
│   │   ├── pages/     # 页面
│   │   └── styles/    # 样式
│   └── package.json   # 前端依赖
├── python/            # Python爬虫
│   ├── 1.py           # 主爬虫脚本
│   ├── funds.json     # 基金数据缓存
│   └── requirements.txt # 爬虫依赖
├── server/            # 后端服务器（Node.js）
│   ├── models/        # MongoDB模型
│   ├── .env           # 环境变量
│   ├── server.js      # 主服务器文件
│   └── package.json   # 后端依赖
└── README.md          # 项目说明
```

## 技术栈

### 前端
- **框架**: Next.js 14
- **语言**: JavaScript
- **样式**: CSS Modules
- **状态管理**: React Hooks

### 后端
- **框架**: Express
- **数据库**: MongoDB
- **认证**: JWT + bcrypt
- **邮件**: SendGrid
- **定时任务**: node-schedule

### 爬虫
- **语言**: Python 3
- **库**: requests, BeautifulSoup4, json

## 功能特性

### 核心功能
1. **基金数据抓取**: 定时从天天基金网抓取50个热门基金数据
2. **实时数据展示**: 前端展示基金名称、代码、涨跌幅等信息
3. **搜索功能**: 支持按基金名称或代码搜索
4. **用户系统**: 注册、登录、邮箱验证
5. **基金标记**: 用户可以标记关注的基金
6. **涨幅提醒**: 当基金涨幅超过阈值时发送邮件通知

### 技术特性
- **缓存机制**: 本地文件缓存 + MongoDB存储
- **异步更新**: 后台定时更新数据，不影响用户体验
- **跨域支持**: 配置CORS，支持前端跨域请求
- **安全认证**: JWT token认证，密码加密存储

## 快速开始

### 1. 环境要求
- Node.js 16+
- Python 3.8+
- MongoDB 4.4+

### 2. 安装依赖

#### 前端
```bash
cd client
npm install
```

#### 后端
```bash
cd server
npm install
```

#### 爬虫
```bash
cd python
pip install requests beautifulsoup4
```

### 3. 配置环境变量

复制 `server/.env.example` 为 `server/.env` 并填写：

```env
# MongoDB连接字符串
MONGODB_URI=mongodb://localhost:27017/fund_tracking

# JWT密钥
JWT_SECRET=your_jwt_secret_key_here

# 邮箱配置（SendGrid）
EMAIL_SERVICE=SendGrid
EMAIL_USER=apikey
EMAIL_PASS=your_sendgrid_api_key
EMAIL_FROM=noreply@fundtracking.com

# 服务器端口
PORT=3001

# 前端URL
CLIENT_URL=http://localhost:3000
```

### 4. 启动服务

#### 启动MongoDB
```bash
# Windows
net start MongoDB

# Linux/Mac
sudo systemctl start mongodb
```

#### 启动后端服务器
```bash
cd server
npm start
```

#### 启动前端开发服务器
```bash
cd client
npm run dev
```

### 5. 访问项目
- **前端**: http://localhost:3000
- **后端API**: http://localhost:3001/api/test

## API 接口

### 认证接口
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/verify` - 邮箱验证
- `POST /api/auth/login` - 用户登录

### 基金接口
- `GET /api/funds/all` - 获取所有基金
- `GET /api/fund/:fundCode` - 获取单个基金

### 关注列表接口
- `GET /api/watchlist` - 获取用户关注列表
- `POST /api/watchlist` - 添加基金到关注列表
- `DELETE /api/watchlist/:fundCode` - 从关注列表移除
- `PUT /api/watchlist/:fundCode` - 更新提醒阈值

## 部署说明

### 本地部署
按照上述快速开始步骤即可。

### 服务器部署
1. **安装依赖**: 安装Node.js、Python、MongoDB
2. **配置环境变量**: 填写生产环境的配置
3. **启动服务**: 使用PM2管理进程
4. **设置定时任务**: 确保MongoDB服务自动启动

## 注意事项

1. **邮箱验证**: 需要配置SendGrid API Key才能发送验证码
2. **爬虫频率**: 默认每30秒抓取一次数据，可根据需要调整
3. **数据安全**: 密码使用bcrypt加密存储，JWT token有效期7天
4. **性能优化**: 本地文件缓存 + 数据库存储，确保响应速度

## 下一步计划

1. **完善用户系统**: 注册登录功能正在开发中，尚未完成
2. **数据库联立**: MongoDB数据库尚未正式联立，目前使用模拟数据
3. **移动端适配**: 响应式设计，支持手机访问
4. **数据可视化**: 添加基金走势图、历史数据图表
5. **多语言支持**: 中英文切换
6. **社交功能**: 分享基金分析结果

## 当前状态

### 已完成
- ✅ **首页UI优化**: 实现了现代化的首页设计
  - 粒子背景效果（黑色背景+白色粒子）
  - 药丸导航组件（白色药丸+黑色文字，hover时黑色背景+白色文字）
  - 基金市场概览展示
  - 响应式卡片布局
- ✅ **药丸导航**: 实现了完整的药丸导航效果
  - GSAP动画库驱动的流畅过渡
  - 默认白色药丸+黑色文字
  - hover时黑色背景覆盖+白色文字
  - 响应式设计，支持移动端
- ✅ **基金数据展示**: 模拟基金数据展示功能
- ✅ **搜索功能**: 支持基金名称和代码搜索

### 进行中
- 🚧 **用户注册功能**: 接下来着手开发注册功能及页面优化
- 🚧 **页面优化**: 持续优化UI/UX体验

### 待开发
- ⏳ **真实数据接口**: 替换模拟数据为真实API
- ⏳ **用户认证**: 完善JWT认证流程
- ⏳ **基金标记功能**: 用户关注列表
- ⏳ **涨幅提醒**: 邮件通知功能
- ⏳ **定时爬虫**: 自动抓取基金数据

## 许可证

MIT License
