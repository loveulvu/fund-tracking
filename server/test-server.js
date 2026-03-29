const http = require('http');

const server = http.createServer((req, res) => {
  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url}`);
  
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
    console.log('Test endpoint hit');
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ message: 'Server is running', timestamp: new Date().toISOString() }));
  } else if (req.url === '/api/funds/all') {
    console.log('All funds endpoint hit');
    // 返回测试数据
    const testData = [
      {
        fund_code: "006030",
        fund_name: "南方昌元可转债债券A",
        one_year_profit: "38.84%",
        six_month_profit: "9.76%",
        three_month_profit: "1.13%",
        one_month_profit: "-10.76%"
      },
      {
        fund_code: "000001",
        fund_name: "华夏成长混合",
        one_year_profit: "15.23%",
        six_month_profit: "8.45%",
        three_month_profit: "3.21%",
        one_month_profit: "-2.15%"
      }
    ];
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(testData));
  } else if (req.url.startsWith('/api/fund/')) {
    const fundCode = req.url.split('/').pop();
    console.log(`Fund endpoint hit with code: ${fundCode}`);
    const testData = {
      fund_code: fundCode,
      fund_name: `基金 ${fundCode}`,
      one_year_profit: "20.00%",
      six_month_profit: "10.00%",
      three_month_profit: "5.00%",
      one_month_profit: "1.00%"
    };
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(testData));
  } else {
    console.log(`404 Not Found: ${req.url}`);
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Not found', path: req.url }));
  }
});

server.listen(3001, () => {
  console.log('Test server running at http://localhost:3001');
  console.log('Available endpoints:');
  console.log('  - GET /api/test');
  console.log('  - GET /api/funds/all');
  console.log('  - GET /api/fund/:fundCode');
});
