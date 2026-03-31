import os
import time
import requests
from bs4 import BeautifulSoup
from flask import Flask, jsonify
from flask_cors import CORS
from pymongo import MongoClient
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# --- 数据库配置 ---
# 优先从 Railway 环境变量读取，没有则报错，确保安全
MONGO_URI = os.getenv("MONGO_URI")
if not MONGO_URI:
    raise ValueError("致命错误：未找到 MONGO_URI 环境变量，请检查配置。")

client = MongoClient(MONGO_URI)
db = client['fund_database']
collection = db['fund_data']

DEFAULT_FUND_CODES = [
    "006030", "000001", "110022", "161725", "110011",
    "001184", "000988", "001048", "001511", "001555",
    "001618", "001630", "001668", "001717", "001809",
    "001838", "001865", "001938", "002001", "002147",
    "002288", "002300", "002311", "002344", "002351",
    "002390", "002407", "002420", "002438", "002441",
    "002442", "002443", "002445", "002446", "000021",
    "000031", "000051", "000061", "000071", "000081",
    "000091", "000101", "000111", "000121", "000131",
    "000141", "000151", "000161", "000171", "000181"
]

def get_fund_info(fund_code):
    url = f"https://fund.eastmoney.com/{fund_code}.html"
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.encoding = 'utf-8'
        soup = BeautifulSoup(response.text, 'html.parser')
        fund_name = soup.find('span', class_='funCur-FundName')
        fund_name = fund_name.text.strip() if fund_name else "未知"
        # 简化抓取逻辑以确保稳定性
        return {
            "fund_code": fund_code,
            "fund_name": fund_name,
            "update_time": int(time.time())
        }
    except Exception:
        return None

@app.route('/api/update')
def update_funds():
    updated = []
    for c in DEFAULT_FUND_CODES[:10]: # 先测试前10个，避免超时
        data = get_fund_info(c)
        if data:
            collection.update_one({"fund_code": c}, {"$set": data}, upsert=True)
            updated.append(c)
    return jsonify({"status": "success", "count": len(updated)})

@app.route('/api/funds')
def get_funds():
    data = list(collection.find({}, {"_id": 0}))
    return jsonify(data)

@app.route('/')
def index():
    return "Fund Tracking API is Running"

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 5000))
    app.run(host='0.0.0.0', port=port)