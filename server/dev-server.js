const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = 3001;
const FUNDS_PATH = path.join(__dirname, '../python/funds.json');

// 读取基金数据
function loadFunds() {
  try {
    const data = fs.readFileSync(FUNDS_PATH, 'utf-8');
    return JSON.parse(data);
  } catch (e) {
    console.error('Error loading funds:', e.message);
    return [];
  }
}

const server = http.createServer((req, res) => {
  // 设置CORS头
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  
  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }
  
  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url}`);
  
  if (req.url === '/api/test') {
    res.writeHead(200);
    res.end(JSON.stringify({ message: 'Server is running' }));
  } else if (req.url === '/api/funds/all') {
    const funds = loadFunds();
    console.log(`Returning ${funds.length} funds`);
    res.writeHead(200);
    res.end(JSON.stringify(funds));
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    const funds = loadFunds();
    const fund = funds.find(f => f.fund_code === fundCode);
    
    if (fund) {
      res.writeHead(200);
      res.end(JSON.stringify(fund));
    } else {
      res.writeHead(404);
      res.end(JSON.stringify({ error: 'Fund not found', fundCode }));
    }
  } else {
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'Not found' }));
  }
});

server.listen(PORT, () => {
  console.log(`Dev server running at http://localhost:${PORT}`);
  console.log('Using cached funds data');
  console.log('Endpoints:');
  console.log('  - GET /api/test');
  console.log('  - GET /api/funds/all');
  console.log('  - GET /api/fund/:fundCode');
});
