const http = require('http');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

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
    res.writeHead(200);
    res.end(JSON.stringify({ message: 'Server is running' }));
  } else if (req.url === '/api/funds/all') {
    console.log('All funds endpoint hit');
    fetchAllFunds(res);
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    console.log(`Fund endpoint hit with code: ${fundCode}`);
    fetchSingleFund(fundCode, res);
  } else {
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'Not found', path: req.url }));
  }
});

// 获取所有基金
function fetchAllFunds(res) {
  const pythonScriptPath = path.join(__dirname, '../python/1.py');
  const pythonCwd = path.join(__dirname, '../python');
  
  console.log(`Running Python script: ${pythonScriptPath}`);
  
  const timeout = setTimeout(() => {
    console.error('Python script timeout');
    if (!res.headersSent) {
      res.writeHead(504);
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }
  }, 120000);
  
  const pythonProcess = spawn('python', [pythonScriptPath, 'all'], { cwd: pythonCwd });
  
  let stdout = '';
  let stderr = '';
  
  pythonProcess.stdout.on('data', (data) => {
    stdout += data.toString();
  });
  
  pythonProcess.stderr.on('data', (data) => {
    stderr += data.toString();
    console.error('Python stderr:', data.toString().substring(0, 100));
  });
  
  pythonProcess.on('close', (code) => {
    clearTimeout(timeout);
    console.log(`Python process exited with code: ${code}`);
    console.log('Python stdout:', stdout);
    
    if (code !== 0) {
      res.writeHead(500);
      res.end(JSON.stringify({ error: 'Python script failed', code, stderr }));
      return;
    }
    
    // 从输出中提取文件路径
    const match = stdout.match(/FUNDS_JSON:(.+)/);
    if (match) {
      const jsonFile = match[1].trim();
      const jsonPath = path.join(pythonCwd, jsonFile);
      console.log(`Reading funds from: ${jsonPath}`);
      
      try {
        const data = fs.readFileSync(jsonPath, 'utf-8');
        const funds = JSON.parse(data);
        console.log(`Successfully read ${funds.length} funds`);
        res.writeHead(200);
        res.end(JSON.stringify(funds));
      } catch (e) {
        console.error('Error reading funds file:', e.message);
        res.writeHead(500);
        res.end(JSON.stringify({ error: 'Failed to read funds file', details: e.message }));
      }
    } else {
      res.writeHead(500);
      res.end(JSON.stringify({ error: 'Invalid Python output', stdout }));
    }
  });
  
  pythonProcess.on('error', (error) => {
    clearTimeout(timeout);
    res.writeHead(500);
    res.end(JSON.stringify({ error: 'Failed to spawn Python', details: error.message }));
  });
}

// 获取单个基金
function fetchSingleFund(fundCode, res) {
  const pythonScriptPath = path.join(__dirname, '../python/1.py');
  const pythonCwd = path.join(__dirname, '../python');
  
  const timeout = setTimeout(() => {
    console.error('Python script timeout');
    if (!res.headersSent) {
      res.writeHead(504);
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }
  }, 30000);
  
  const pythonProcess = spawn('python', [pythonScriptPath, fundCode], { cwd: pythonCwd });
  
  let stdout = '';
  let stderr = '';
  
  pythonProcess.stdout.on('data', (data) => {
    stdout += data.toString();
  });
  
  pythonProcess.stderr.on('data', (data) => {
    stderr += data.toString();
  });
  
  pythonProcess.on('close', (code) => {
    clearTimeout(timeout);
    
    if (code !== 0) {
      res.writeHead(500);
      res.end(JSON.stringify({ error: 'Python script failed', code, stderr }));
      return;
    }
    
    // 从输出中提取文件路径
    const match = stdout.match(/FUND_JSON:(.+)/);
    if (match) {
      const jsonFile = match[1].trim();
      const jsonPath = path.join(pythonCwd, jsonFile);
      
      try {
        const data = fs.readFileSync(jsonPath, 'utf-8');
        const fund = JSON.parse(data);
        res.writeHead(200);
        res.end(JSON.stringify(fund));
      } catch (e) {
        res.writeHead(500);
        res.end(JSON.stringify({ error: 'Failed to read fund file', details: e.message }));
      }
    } else {
      res.writeHead(500);
      res.end(JSON.stringify({ error: 'Invalid Python output', stdout }));
    }
  });
  
  pythonProcess.on('error', (error) => {
    clearTimeout(timeout);
    res.writeHead(500);
    res.end(JSON.stringify({ error: 'Failed to spawn Python', details: error.message }));
  });
}

server.listen(3001, () => {
  console.log('File server running at http://localhost:3001');
  console.log('Available endpoints:');
  console.log('  - GET /api/test');
  console.log('  - GET /api/funds/all');
  console.log('  - GET /api/fund/:fundCode');
});
