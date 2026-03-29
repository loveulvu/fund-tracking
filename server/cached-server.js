const http = require('http');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// 读取缓存的基金数据
let cachedFunds = [];
const fundsJsonPath = path.join(__dirname, '../python/funds.json');

// 尝试读取缓存数据
if (fs.existsSync(fundsJsonPath)) {
  try {
    const data = fs.readFileSync(fundsJsonPath, 'utf-8');
    cachedFunds = JSON.parse(data);
    console.log(`Loaded ${cachedFunds.length} funds from cache`);
  } catch (e) {
    console.error('Error loading cached funds:', e.message);
  }
}

const server = http.createServer((req, res) => {
  console.log(`\n[${new Date().toISOString()}] ${req.method} ${req.url}`);
  
  // 添加CORS头
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  
  // 处理OPTIONS请求
  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }
  
  if (req.url === '/api/test') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ message: 'Server is running', timestamp: new Date().toISOString() }));
  } else if (req.url === '/api/funds/all') {
    console.log('All funds endpoint hit');
    
    // 如果有缓存数据，先返回缓存
    if (cachedFunds.length > 0) {
      console.log(`Returning ${cachedFunds.length} cached funds`);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(cachedFunds));
      
      // 后台更新数据（异步）
      updateFundsInBackground();
    } else {
      // 没有缓存，同步获取
      fetchAllFunds(res);
    }
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    console.log(`Fund endpoint hit with code: ${fundCode}`);
    fetchSingleFund(fundCode, res);
  } else {
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Not found', path: req.url }));
  }
});

// 后台更新基金数据
function updateFundsInBackground() {
  console.log('Updating funds in background...');
  const pythonScriptPath = path.join(__dirname, '../python/1.py');
  const pythonProcess = spawn('python', [pythonScriptPath, 'all']);
  
  let stdout = '';
  
  pythonProcess.stdout.on('data', (data) => {
    stdout += data.toString('utf-8');
  });
  
  pythonProcess.on('close', (code) => {
    if (code === 0 && stdout.trim()) {
      try {
        const funds = JSON.parse(stdout);
        cachedFunds = funds;
        // 保存到文件
        fs.writeFileSync(fundsJsonPath, stdout);
        console.log(`Updated cache with ${funds.length} funds`);
      } catch (e) {
        console.error('Error parsing updated funds:', e.message);
      }
    }
  });
}

// 同步获取所有基金
function fetchAllFunds(res) {
  const pythonScriptPath = path.join(__dirname, '../python/1.py');
  console.log(`Python script path: ${pythonScriptPath}`);
  
  // 设置超时
  const timeout = setTimeout(() => {
    console.error('Python script timeout');
    if (!res.headersSent) {
      res.writeHead(504, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }
  }, 120000);
  
  const pythonProcess = spawn('python', [pythonScriptPath, 'all']);
  
  let stdout = '';
  let stderr = '';
  
  pythonProcess.stdout.on('data', (data) => {
    stdout += data.toString('utf-8');
  });
  
  pythonProcess.stderr.on('data', (data) => {
    stderr += data.toString();
    console.error('Python stderr:', data.toString().substring(0, 100));
  });
  
  pythonProcess.on('close', (code) => {
    clearTimeout(timeout);
    console.log(`Python process exited with code: ${code}`);
    
    if (code !== 0) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script failed', code, stderr }));
      return;
    }
    
    if (!stdout.trim()) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'No output from Python script' }));
      return;
    }
    
    try {
      const funds = JSON.parse(stdout);
      cachedFunds = funds;
      // 保存到文件
      fs.writeFileSync(fundsJsonPath, stdout);
      console.log(`Fetched ${funds.length} funds`);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(funds));
    } catch (e) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Failed to parse JSON', details: e.message }));
    }
  });
  
  pythonProcess.on('error', (error) => {
    clearTimeout(timeout);
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Failed to spawn Python', details: error.message }));
  });
}

// 获取单个基金
function fetchSingleFund(fundCode, res) {
  const pythonScriptPath = path.join(__dirname, '../python/1.py');
  
  const timeout = setTimeout(() => {
    console.error('Python script timeout');
    if (!res.headersSent) {
      res.writeHead(504, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }
  }, 30000);
  
  const pythonProcess = spawn('python', [pythonScriptPath, fundCode]);
  
  let stdout = '';
  let stderr = '';
  
  pythonProcess.stdout.on('data', (data) => {
    stdout += data.toString('utf-8');
  });
  
  pythonProcess.stderr.on('data', (data) => {
    stderr += data.toString();
  });
  
  pythonProcess.on('close', (code) => {
    clearTimeout(timeout);
    
    if (code !== 0) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script failed', code, stderr }));
      return;
    }
    
    if (!stdout.trim()) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'No output from Python script' }));
      return;
    }
    
    try {
      const fund = JSON.parse(stdout);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(fund));
    } catch (e) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Failed to parse JSON', details: e.message }));
    }
  });
  
  pythonProcess.on('error', (error) => {
    clearTimeout(timeout);
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Failed to spawn Python', details: error.message }));
  });
}

server.listen(3001, () => {
  console.log('Cached server running at http://localhost:3001');
  console.log(`Loaded ${cachedFunds.length} funds from cache`);
  console.log('Available endpoints:');
  console.log('  - GET /api/test');
  console.log('  - GET /api/funds/all');
  console.log('  - GET /api/fund/:fundCode');
});
