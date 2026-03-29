const http = require('http');
const { spawn } = require('child_process');
const path = require('path');

const server = http.createServer((req, res) => {
  // 添加CORS头
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  
  // 处理OPTIONS请求
  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }
  
  if (req.url === '/api/test') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ message: 'Server is running' }));
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    console.log(`Fetching fund data for code: ${fundCode}`);
    
    // 执行Python脚本获取基金数据
    const pythonScriptPath = path.join(__dirname, '../python/1.py');
    console.log(`Python script path: ${pythonScriptPath}`);
    
    // 使用spawn执行Python脚本
    const pythonProcess = spawn('python', [pythonScriptPath, fundCode]);
    
    let stdout = '';
    let stderr = '';
    
    pythonProcess.stdout.on('data', (data) => {
      stdout += data.toString();
      console.log('Python stdout chunk:', data.toString());
    });
    
    pythonProcess.stderr.on('data', (data) => {
      stderr += data.toString();
      console.error('Python stderr chunk:', data.toString());
    });
    
    pythonProcess.on('close', (code) => {
      console.log(`Python process exited with code: ${code}`);
      console.log('Final stdout length:', stdout.length);
      console.log('Final stdout content:', stdout);
      console.log('Final stderr:', stderr);
      
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
        console.log('Parsed fund data:', fundData);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(fundData));
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        console.error('Raw output:', stdout);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout 
        }));
      }
    });
    
    pythonProcess.on('error', (error) => {
      console.error('Error spawning Python process:', error.message);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      }));
    });
  } else if (req.url === '/api/funds/all') {
    console.log('Fetching all funds data');
    
    // 执行Python脚本获取所有基金数据
    const pythonScriptPath = path.join(__dirname, '../python/1.py');
    console.log(`Python script path: ${pythonScriptPath}`);
    
    // 使用spawn执行Python脚本，传入'all'参数
    const pythonProcess = spawn('python', [pythonScriptPath, 'all']);
    
    let stdout = '';
    let stderr = '';
    
    pythonProcess.stdout.on('data', (data) => {
      stdout += data.toString();
      console.log('Python stdout chunk:', data.toString());
    });
    
    pythonProcess.stderr.on('data', (data) => {
      stderr += data.toString();
      console.error('Python stderr chunk:', data.toString());
    });
    
    pythonProcess.on('close', (code) => {
      console.log(`Python process exited with code: ${code}`);
      console.log('Final stdout length:', stdout.length);
      console.log('Final stdout content:', stdout);
      console.log('Final stderr:', stderr);
      
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
        console.log('Parsed funds data:', fundsData);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(fundsData));
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        console.error('Raw output:', stdout);
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout 
        }));
      }
    });
    
    pythonProcess.on('error', (error) => {
      console.error('Error spawning Python process:', error.message);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      }));
    });
  } else {
    res.writeHead(404);
    res.end('Not found');
  }
});

server.listen(3001, () => {
  console.log('Simple server running at http://localhost:3001');
  console.log('Test endpoint: http://localhost:3001/api/test');
  console.log('Fund data endpoint: http://localhost:3001/api/fund/:fundCode');
  console.log('All funds endpoint: http://localhost:3001/api/funds/all');
});
