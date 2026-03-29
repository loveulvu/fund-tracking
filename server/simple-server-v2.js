const http = require('http');
const { spawn } = require('child_process');
const path = require('path');

const server = http.createServer((req, res) => {
  console.log(`\n[${new Date().toISOString()}] ${req.method} ${req.url}`);
  
  // 添加CORS头
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  
  // 处理OPTIONS请求
  if (req.method === 'OPTIONS') {
    console.log('Handling OPTIONS request');
    res.writeHead(204);
    res.end();
    return;
  }
  
  if (req.url === '/api/test') {
    console.log('Test endpoint hit');
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ message: 'Server is running', timestamp: new Date().toISOString() }));
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    console.log(`Fund endpoint hit with code: ${fundCode}`);
    
    // 执行Python脚本获取基金数据
    const pythonScriptPath = path.join(__dirname, '../python/1.py');
    console.log(`Python script path: ${pythonScriptPath}`);
    
    // 设置超时
    const timeout = setTimeout(() => {
      console.error('Python script timeout');
      res.writeHead(504, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }, 30000); // 30秒超时
    
    // 使用spawn执行Python脚本
    const pythonProcess = spawn('python', [pythonScriptPath, fundCode]);
    
    let stdout = '';
    let stderr = '';
    
    pythonProcess.stdout.on('data', (data) => {
      stdout += data.toString();
      console.log('Python stdout chunk:', data.toString().substring(0, 100));
    });
    
    pythonProcess.stderr.on('data', (data) => {
      stderr += data.toString();
      console.error('Python stderr chunk:', data.toString().substring(0, 100));
    });
    
    pythonProcess.on('close', (code) => {
      clearTimeout(timeout);
      console.log(`Python process exited with code: ${code}`);
      console.log('Final stdout length:', stdout.length);
      
      if (code !== 0) {
        console.error('Python script failed with code:', code);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Python script failed', 
          code: code, 
          stderr: stderr 
        }));
        return;
      }
      
      if (!stdout || stdout.trim() === '') {
        console.error('No output from Python script');
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'No output from Python script' }));
        return;
      }
      
      try {
        // 解析输出（假设输出是JSON格式）
        const fundData = JSON.parse(stdout);
        console.log('Parsed fund data successfully');
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(fundData));
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout.substring(0, 200)
        }));
      }
    });
    
    pythonProcess.on('error', (error) => {
      clearTimeout(timeout);
      console.error('Error spawning Python process:', error.message);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      }));
    });
  } else if (req.url === '/api/funds/all') {
    console.log('All funds endpoint hit');
    
    // 执行Python脚本获取所有基金数据
    const pythonScriptPath = path.join(__dirname, '../python/1.py');
    console.log(`Python script path: ${pythonScriptPath}`);
    
    // 设置超时（50个基金可能需要更长时间）
    const timeout = setTimeout(() => {
      console.error('Python script timeout');
      res.writeHead(504, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Python script timeout' }));
    }, 120000); // 120秒超时
    
    // 使用spawn执行Python脚本，传入'all'参数
    const pythonProcess = spawn('python', [pythonScriptPath, 'all']);
    
    let stdout = '';
    let stderr = '';
    
    pythonProcess.stdout.on('data', (data) => {
      stdout += data.toString();
      console.log('Python stdout chunk length:', data.toString().length);
    });
    
    pythonProcess.stderr.on('data', (data) => {
      stderr += data.toString();
      console.error('Python stderr chunk:', data.toString().substring(0, 100));
    });
    
    pythonProcess.on('close', (code) => {
      clearTimeout(timeout);
      console.log(`Python process exited with code: ${code}`);
      console.log('Final stdout length:', stdout.length);
      
      if (code !== 0) {
        console.error('Python script failed with code:', code);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Python script failed', 
          code: code, 
          stderr: stderr 
        }));
        return;
      }
      
      if (!stdout || stdout.trim() === '') {
        console.error('No output from Python script');
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'No output from Python script' }));
        return;
      }
      
      try {
        // 解析输出（假设输出是JSON格式）
        const fundsData = JSON.parse(stdout);
        console.log('Parsed funds data successfully, count:', fundsData.length);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(fundsData));
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout.substring(0, 200)
        }));
      }
    });
    
    pythonProcess.on('error', (error) => {
      clearTimeout(timeout);
      console.error('Error spawning Python process:', error.message);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      }));
    });
  } else {
    console.log(`404 Not Found: ${req.url}`);
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Not found', path: req.url }));
  }
});

server.listen(3001, () => {
  console.log('Simple server v2 running at http://localhost:3001');
  console.log('Available endpoints:');
  console.log('  - GET /api/test');
  console.log('  - GET /api/funds/all');
  console.log('  - GET /api/fund/:fundCode');
});
