import os
import time
import requests
from bs4 import BeautifulSoup
from flask import Flask, jsonify
from flask_cors import CORS
from pymongo import MongoClient

app = Flask(__name__)
CORS(app)

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")

# 2. 全局数据库对象占位
db = None
collection = None
db_error_message = None

# 3. 柔性连接逻辑：即使失败也不触发进程崩溃，保证 Railway 能顺利启动
if not MONGO_URI:
    db_error_message = "CRITICAL: MONGO_URI 环境变量缺失，请在 Railway 的 Variables 中添加。"
    print(db_error_message)
else:
    try:
        # 设置 2 秒超时，防止网络卡死导致 Railway 健康检查超时
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=2000)
        db = client['fund_database']
        collection = db['fund_data']
        # 测试连接
        client.admin.command('ping')
        print("MongoDB 连接成功。")
    except Exception as e:
        db_error_message = f"MongoDB 连接失败: {str(e)}"
        print(db_error_message)

DEFAULT_FUND_CODES = [
    "006030", "000001", "110022", "161725", "110011"
]

def get_fund_info(fund_code):
    url = f"https://fund.eastmoney.com/{fund_code}.html"
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    try:
        response = requests.get(url, headers=headers, timeout=5)
        response.encoding = 'utf-8'
        soup = BeautifulSoup(response.text, 'html.parser')
        fund_name = soup.find('span', class_='funCur-FundName')
        fund_name = fund_name.text.strip() if fund_name else "未知"
        return {
            "fund_code": fund_code,
            "fund_name": fund_name,
            "update_time": int(time.time())
        }
    except Exception:
        return None

# API 路由安全检查装饰逻辑
def check_db_status():
    if db_error_message:
        return jsonify({"status": "error", "message": db_error_message}), 500
    if collection is None:
        return jsonify({"status": "error", "message": "数据库未初始化"}), 500
    return None

@app.route('/api/update')
def update_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    updated = []
    for c in DEFAULT_FUND_CODES:
        data = get_fund_info(c)
        if data:
            collection.update_one({"fund_code": c}, {"$set": data}, upsert=True)
            updated.append(c)
    return jsonify({"status": "success", "count": len(updated), "codes": updated})

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    data = list(collection.find({}, {"_id": 0}))
    return jsonify(data)

@app.route('/')
def index():
    if db_error_message:
        return f"API is Running, but Database Error: {db_error_message}", 200
    return "Fund Tracking API is Running successfully with DB connected.", 200

if __name__ == "__main__":
    # Railway 必须动态读取 PORT 变量
    port = int(os.environ.get("PORT", 5000))
    app.run(host='0.0.0.0', port=port)