const express = require('express');
const cors = require('cors');
const { spawn } = require('child_process');
const path = require('path');

const app = express();
const port = 3001;

// 启用CORS
app.use(cors());

// 测试端点
app.get('/api/test', (req, res) => {
  res.json({ message: 'Server is running' });
});

// API端点：获取单个基金数据
app.get('/api/fund/:fundCode', (req, res) => {
  try {
    const fundCode = req.params.fundCode;
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
        res.status(500).json({ 
          error: 'Python script failed', 
          code: code, 
          stderr: stderr 
        });
        return;
      }
      
      if (!stdout || stdout.trim() === '') {
        console.error('No output from Python script');
        res.status(500).json({ error: 'No output from Python script' });
        return;
      }
      
      try {
        // 解析输出（假设输出是JSON格式）
        const fundData = JSON.parse(stdout);
        console.log('Parsed fund data:', fundData);
        res.json(fundData);
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        console.error('Raw output:', stdout);
        res.status(500).json({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout 
        });
      }
    });
    
    pythonProcess.on('error', (error) => {
      console.error('Error spawning Python process:', error.message);
      res.status(500).json({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      });
    });
    
  } catch (error) {
    console.error('Error:', error.message);
    res.status(500).json({ error: 'Failed to get fund data', details: error.message });
  }
});

// API端点：获取所有基金数据
app.get('/api/funds/all', (req, res) => {
  try {
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
        res.status(500).json({ 
          error: 'Python script failed', 
          code: code, 
          stderr: stderr 
        });
        return;
      }
      
      if (!stdout || stdout.trim() === '') {
        console.error('No output from Python script');
        res.status(500).json({ error: 'No output from Python script' });
        return;
      }
      
      try {
        // 解析输出（假设输出是JSON格式）
        const fundsData = JSON.parse(stdout);
        console.log('Parsed funds data:', fundsData);
        res.json(fundsData);
      } catch (parseError) {
        console.error('JSON parse error:', parseError.message);
        console.error('Raw output:', stdout);
        res.status(500).json({ 
          error: 'Failed to parse JSON output', 
          details: parseError.message, 
          rawOutput: stdout 
        });
      }
    });
    
    pythonProcess.on('error', (error) => {
      console.error('Error spawning Python process:', error.message);
      res.status(500).json({ 
        error: 'Failed to spawn Python process', 
        details: error.message 
      });
    });
    
  } catch (error) {
    console.error('Error:', error.message);
    res.status(500).json({ error: 'Failed to get funds data', details: error.message });
  }
});

// 启动服务器
app.listen(port, () => {
  console.log(`Server running at http://localhost:${port}`);
  console.log('Test endpoint: http://localhost:3001/api/test');
  console.log('Fund data endpoint: http://localhost:3001/api/fund/:fundCode');
});
