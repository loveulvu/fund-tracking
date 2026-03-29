const http = require('http');
const fs = require('fs');
const path = require('path');

// 测试中文输出
const testData = [
  {
    fund_code: "006030",
    fund_name: "南方昌元可转债债券A",
    one_year_profit: "38.84%"
  },
  {
    fund_code: "000001", 
    fund_name: "华夏成长混合",
    one_year_profit: "20.22%"
  }
];

const server = http.createServer((req, res) => {
  console.log(`Request: ${req.url}`);
  
  // 设置响应头
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  
  if (req.url === '/api/test-encoding') {
    // 直接返回测试数据
    res.writeHead(200);
    res.end(JSON.stringify(testData));
  } else if (req.url === '/api/test-file') {
    // 读取文件测试
    const fundsJsonPath = path.join(__dirname, '../python/funds.json');
    try {
      const data = fs.readFileSync(fundsJsonPath, 'utf-8');
      const funds = JSON.parse(data);
      // 只返回前2个
      res.writeHead(200);
      res.end(JSON.stringify(funds.slice(0, 2)));
    } catch (e) {
      res.writeHead(500);
      res.end(JSON.stringify({ error: e.message }));
    }
  } else {
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'Not found' }));
  }
});

server.listen(3002, () => {
  console.log('Test server running at http://localhost:3002');
  console.log('Endpoints:');
  console.log('  - GET /api/test-encoding (hardcoded data)');
  console.log('  - GET /api/test-file (read from file)');
});
