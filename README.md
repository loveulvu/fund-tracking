# 基金追踪系统 (Fund Tracking System)

一个完整的基金数据追踪和分析系统，支持实时抓取基金数据、用户注册登录、基金关注和涨幅提醒功能。

## 项目结构

```
├── client/            # 前端项目（Next.js）
│   ├── public/        # 静态资源
│   ├── src/           # 源代码
│   │   ├── components/ # 组件
│   │   ├── pages/     # 页面
│   │   └── styles/    # 样式
│   └── package.json   # 前端依赖
├── server/            # 后端服务器（Node.js）
│   ├── models/        # MongoDB模型
│   ├── .env           # 环境变量
│   ├── server.js      # 主服务器文件
│   └── package.json   # 后端依赖
├── app.py             # Flask后端（Python）
├── requirements.txt   # Python依赖
├── Procfile           # Railway部署配置
└── README.md          # 项目说明
```

## 技术栈

### 前端
- **框架**: Next.js 14
- **语言**: JavaScript
- **样式**: CSS Modules
- **状态管理**: React Hooks

### 后端
- **框架**: Flask (Python)
- **数据库**: MongoDB
- **认证**: JWT
- **邮件**: QQ邮箱SMTP

### 爬虫
- **语言**: Python 3
- **库**: requests, BeautifulSoup4

## 功能特性

### 核心功能
1. **基金数据抓取**: 从天天基金网实时抓取基金数据
2. **实时数据展示**: 前端展示基金名称、代码、净值、涨跌幅、收益数据
3. **搜索功能**: 支持按基金名称或代码搜索，自动从天天基金网获取数据
4. **用户系统**: 注册、登录、邮箱验证
5. **基金关注**: 用户可以关注/取消关注基金
6. **涨幅提醒**: 设置阈值，当基金涨幅超过阈值时发送邮件通知

### 技术特性
- **缓存机制**: MongoDB存储基金数据
- **异步更新**: 后台定时更新数据，不影响用户体验
- **跨域支持**: 配置CORS，支持前端跨域请求
- **安全认证**: JWT token认证，密码加密存储

## 部署信息

### 后端部署
- **平台**: Railway
- **地址**: https://fund-tracking-production.up.railway.app
- **端口**: 8080

### 前端部署
- **本地开发**: http://localhost:3000

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
pip install -r requirements.txt
```

### 3. 配置环境变量

创建 `.env` 文件并填写：

```env
# MongoDB连接字符串
MONGO_URI=mongodb://localhost:27017/fund_tracking

# JWT密钥
JWT_SECRET=your_jwt_secret_key_here

# 邮箱配置（QQ邮箱）
EMAIL_HOST=smtp.qq.com
EMAIL_PORT=587
EMAIL_USER=your_email@qq.com
EMAIL_PASS=your_authorization_code
EMAIL_FROM=your_email@qq.com

# 服务器端口
PORT=8080
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
python app.py
```

#### 启动前端开发服务器
```bash
cd client
npm run dev
```

### 5. 访问项目
- **前端**: http://localhost:3000
- **后端API**: https://fund-tracking-production.up.railway.app

## API 接口

### 认证接口
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/verify` - 邮箱验证
- `POST /api/auth/login` - 用户登录

### 基金接口
- `GET /api/funds` - 获取所有基金
- `GET /api/fund/:fundCode` - 获取单个基金
- `GET /api/update` - 更新基金数据

### 关注列表接口
- `GET /api/watchlist` - 获取用户关注列表
- `POST /api/watchlist` - 添加基金到关注列表
- `DELETE /api/watchlist/:fundCode` - 从关注列表移除
- `PUT /api/watchlist/:fundCode` - 更新提醒阈值

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
- ✅ **用户系统**: 完整的注册登录功能
  - 邮箱验证码发送
  - JWT认证
  - 密码加密存储
- ✅ **基金数据展示**: 完整的基金数据展示
  - 基金名称、代码、净值
  - 日涨跌幅（颜色区分涨跌）
  - 近1月、近3月、近6月、近1年收益数据
- ✅ **搜索功能**: 支持基金名称和代码搜索
  - 自动从天天基金网获取数据
  - 实时更新数据库
- ✅ **关注功能**: 用户关注基金功能
  - 添加/取消关注
  - 关注列表展示
  - 阈值设置
- ✅ **邮件提醒**: 涨幅超过阈值时发送邮件通知
- ✅ **Railway部署**: 后端已部署到Railway平台

### 进行中
- 🚧 **页面优化**: 持续优化UI/UX体验

### 待开发
- ⏳ **数据可视化**: 添加基金走势图、历史数据图表
- ⏳ **多语言支持**: 中英文切换
- ⏳ **移动端适配**: 响应式设计优化

## 许可证

MIT License
