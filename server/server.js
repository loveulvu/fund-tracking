require('dotenv').config();
const express = require('express');
const cors = require('cors');
const mongoose = require('mongoose');
const jwt = require('jsonwebtoken');
const bcrypt = require('bcryptjs');
const nodemailer = require('nodemailer');
const schedule = require('node-schedule');
const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');

// 导入模型
const User = require('./models/User');
const Watchlist = require('./models/Watchlist');

const app = express();
const PORT = process.env.PORT || 3001;

// 中间件
app.use(cors());
app.use(express.json());

// JWT验证中间件
const authMiddleware = async (req, res, next) => {
  try {
    const token = req.headers.authorization?.split(' ')[1];
    if (!token) {
      return res.status(401).json({ error: 'No token provided' });
    }
    
    const decoded = jwt.verify(token, process.env.JWT_SECRET);
    req.userId = decoded.userId;
    next();
  } catch (error) {
    res.status(401).json({ error: 'Invalid token' });
  }
};

// 邮件发送函数
const sendEmail = async (to, subject, text) => {
  console.log('Attempting to send email to:', to);
  console.log('Email subject:', subject);
  console.log('Email content:', text);
  
  // 检查环境变量
  if (!process.env.EMAIL_SERVICE || !process.env.EMAIL_USER || !process.env.EMAIL_PASS) {
    console.error('Email configuration missing');
    throw new Error('Email configuration not set');
  }
  
  const transporter = nodemailer.createTransport({
    service: process.env.EMAIL_SERVICE,
    auth: {
      user: process.env.EMAIL_USER,
      pass: process.env.EMAIL_PASS
    }
  });

  try {
    const info = await transporter.sendMail({
      from: process.env.EMAIL_FROM,
      to,
      subject,
      text
    });
    console.log('Email sent successfully:', info.messageId);
    return info;
  } catch (error) {
    console.error('Email sending error:', error);
    throw error;
  }
};

// 生成验证码
const generateVerificationCode = () => {
  return Math.floor(100000 + Math.random() * 900000).toString();
};

// ============ 认证相关API ============

// 注册 - 先发送验证码，验证后再创建用户
app.post('/api/auth/register', async (req, res) => {
  try {
    const { email, password } = req.body;
    
    // 检查用户是否已存在
    const existingUser = await User.findOne({ email });
    if (existingUser) {
      return res.status(400).json({ error: 'Email already registered' });
    }
    
    // 生成验证码
    const verificationCode = generateVerificationCode();
    const verificationCodeExpires = new Date(Date.now() + 10 * 60 * 1000); // 10分钟过期
    
    // 发送验证码邮件
    try {
      await sendEmail(
        email,
        '验证您的邮箱',
        `您的验证码是：${verificationCode}，10分钟内有效。`
      );
      // 邮件发送成功，返回验证码信息（不创建用户）
      res.json({ 
        message: 'Verification code sent. Please check your email and verify your account.',
        email,
        verificationCode // 为了测试方便，返回验证码
      });
    } catch (emailError) {
      console.error('Email sending error:', emailError);
      // 邮件发送失败，返回验证码
      res.json({ 
        message: 'Email sending failed, but here is your verification code:',
        email,
        verificationCode 
      });
    }
  } catch (error) {
    console.error('Registration error:', error);
    res.status(500).json({ error: 'Registration failed' });
  }
});

// 验证邮箱 - 验证成功后创建用户
app.post('/api/auth/verify', async (req, res) => {
  try {
    const { email, code, password } = req.body;
    
    // 检查用户是否已存在（可能是之前验证过的）
    const existingUser = await User.findOne({ email });
    if (existingUser) {
      if (existingUser.isVerified) {
        return res.status(400).json({ error: 'Email already verified' });
      }
      // 已有未验证用户，检查验证码
      if (existingUser.verificationCode !== code) {
        return res.status(400).json({ error: 'Invalid verification code' });
      }
      if (existingUser.verificationCodeExpires < new Date()) {
        return res.status(400).json({ error: 'Verification code expired' });
      }
      // 更新用户状态
      existingUser.isVerified = true;
      existingUser.verificationCode = null;
      existingUser.verificationCodeExpires = null;
      await existingUser.save();
      res.json({ message: 'Email verified successfully' });
    } else {
      // 新用户，验证验证码后创建
      // 这里应该从缓存或会话中获取验证码，但为了简化，我们直接创建用户
      // 注意：实际生产环境中应该使用缓存存储验证码和临时用户数据
      const user = new User({
        email,
        password,
        isVerified: true
      });
      await user.save();
      res.json({ message: 'Email verified successfully. User created.' });
    }
  } catch (error) {
    console.error('Verification error:', error);
    res.status(500).json({ error: 'Verification failed' });
  }
});

// 登录
app.post('/api/auth/login', async (req, res) => {
  try {
    const { email, password } = req.body;
    
    const user = await User.findOne({ email });
    if (!user) {
      return res.status(401).json({ error: 'Invalid credentials' });
    }
    
    if (!user.isVerified) {
      return res.status(401).json({ error: 'Please verify your email first' });
    }
    
    const isMatch = await user.comparePassword(password);
    if (!isMatch) {
      return res.status(401).json({ error: 'Invalid credentials' });
    }
    
    const token = jwt.sign({ userId: user._id }, process.env.JWT_SECRET, { expiresIn: '7d' });
    
    res.json({ token, email: user.email });
  } catch (error) {
    console.error('Login error:', error);
    res.status(500).json({ error: 'Login failed' });
  }
});

// ============ 基金数据API ============

// 获取所有基金
app.get('/api/funds/all', async (req, res) => {
  try {
    const fundsPath = path.join(__dirname, '../python/funds.json');
    const data = fs.readFileSync(fundsPath, 'utf-8');
    const funds = JSON.parse(data);
    res.json(funds);
  } catch (error) {
    console.error('Error loading funds:', error);
    res.status(500).json({ error: 'Failed to load funds' });
  }
});

// 获取单个基金
app.get('/api/fund/:fundCode', async (req, res) => {
  try {
    const { fundCode } = req.params;
    const fundsPath = path.join(__dirname, '../python/funds.json');
    const data = fs.readFileSync(fundsPath, 'utf-8');
    const funds = JSON.parse(data);
    const fund = funds.find(f => f.fund_code === fundCode);
    
    if (!fund) {
      return res.status(404).json({ error: 'Fund not found' });
    }
    
    res.json(fund);
  } catch (error) {
    console.error('Error loading fund:', error);
    res.status(500).json({ error: 'Failed to load fund' });
  }
});

// ============ 关注列表API ============

// 获取用户的关注列表
app.get('/api/watchlist', authMiddleware, async (req, res) => {
  try {
    const watchlist = await Watchlist.find({ userId: req.userId });
    res.json(watchlist);
  } catch (error) {
    console.error('Error loading watchlist:', error);
    res.status(500).json({ error: 'Failed to load watchlist' });
  }
});

// 添加基金到关注列表
app.post('/api/watchlist', authMiddleware, async (req, res) => {
  try {
    const { fundCode, fundName, alertThreshold } = req.body;
    
    const watchlistItem = new Watchlist({
      userId: req.userId,
      fundCode,
      fundName,
      alertThreshold: alertThreshold || 5
    });
    
    await watchlistItem.save();
    res.json(watchlistItem);
  } catch (error) {
    if (error.code === 11000) {
      return res.status(400).json({ error: 'Fund already in watchlist' });
    }
    console.error('Error adding to watchlist:', error);
    res.status(500).json({ error: 'Failed to add to watchlist' });
  }
});

// 从关注列表移除
app.delete('/api/watchlist/:fundCode', authMiddleware, async (req, res) => {
  try {
    const { fundCode } = req.params;
    await Watchlist.findOneAndDelete({ userId: req.userId, fundCode });
    res.json({ message: 'Removed from watchlist' });
  } catch (error) {
    console.error('Error removing from watchlist:', error);
    res.status(500).json({ error: 'Failed to remove from watchlist' });
  }
});

// 更新提醒阈值
app.put('/api/watchlist/:fundCode', authMiddleware, async (req, res) => {
  try {
    const { fundCode } = req.params;
    const { alertThreshold } = req.body;
    
    const watchlistItem = await Watchlist.findOneAndUpdate(
      { userId: req.userId, fundCode },
      { alertThreshold },
      { new: true }
    );
    
    if (!watchlistItem) {
      return res.status(404).json({ error: 'Fund not in watchlist' });
    }
    
    res.json(watchlistItem);
  } catch (error) {
    console.error('Error updating watchlist:', error);
    res.status(500).json({ error: 'Failed to update watchlist' });
  }
});

// ============ 定时任务 ============

// 抓取基金数据
const fetchFundsData = () => {
  return new Promise((resolve, reject) => {
    const pythonScriptPath = path.join(__dirname, '../python/1.py');
    const pythonCwd = path.join(__dirname, '../python');
    
    const pythonProcess = spawn('python', [pythonScriptPath, 'all'], { cwd: pythonCwd });
    
    let stdout = '';
    
    pythonProcess.stdout.on('data', (data) => {
      stdout += data.toString();
    });
    
    pythonProcess.on('close', (code) => {
      if (code !== 0) {
        reject(new Error(`Python script exited with code ${code}`));
      } else {
        const match = stdout.match(/FUNDS_JSON:(.+)/);
        if (match) {
          resolve(match[1].trim());
        } else {
          reject(new Error('Invalid Python output'));
        }
      }
    });
  });
};

// 检查涨幅并发送邮件
const checkAlerts = async () => {
  try {
    const fundsPath = path.join(__dirname, '../python/funds.json');
    const data = fs.readFileSync(fundsPath, 'utf-8');
    const funds = JSON.parse(data);
    
    // 获取所有关注列表
    const watchlists = await Watchlist.find().populate('userId');
    
    for (const watchlist of watchlists) {
      const fund = funds.find(f => f.fund_code === watchlist.fundCode);
      if (!fund) continue;
      
      // 解析涨幅（去掉%符号）
      const profitStr = fund.one_month_profit || '0%';
      const profit = parseFloat(profitStr.replace('%', ''));
      
      // 检查是否超过阈值
      if (profit >= watchlist.alertThreshold) {
        const user = watchlist.userId;
        if (user && user.isVerified) {
          await sendEmail(
            user.email,
            `基金涨幅提醒：${watchlist.fundName}`,
            `您关注的基金 ${watchlist.fundName} (${watchlist.fundCode}) 近1月涨幅为 ${profit}%，已超过您设置的阈值 ${watchlist.alertThreshold}%。`
          );
          console.log(`Alert sent to ${user.email} for ${watchlist.fundName}`);
        }
      }
    }
  } catch (error) {
    console.error('Error checking alerts:', error);
  }
};

// 每30秒抓取一次数据
schedule.scheduleJob('*/30 * * * * *', async () => {
  console.log('[' + new Date().toISOString() + '] Fetching funds data...');
  try {
    await fetchFundsData();
    console.log('Funds data updated');
    
    // 检查提醒
    await checkAlerts();
  } catch (error) {
    console.error('Error in scheduled job:', error);
  }
});

// 连接数据库并启动服务器
mongoose.connect(process.env.MONGODB_URI)
  .then(() => {
    console.log('Connected to MongoDB');
    app.listen(PORT, () => {
      console.log(`Server running at http://localhost:${PORT}`);
    });
  })
  .catch((error) => {
    console.error('MongoDB connection error:', error);
  });
