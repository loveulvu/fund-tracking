import os
import time
import requests
from bs4 import BeautifulSoup
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from pymongo.server_api import ServerApi

app = Flask(__name__)
# 必须启用 CORS，否则 Vercel 上的前端无法访问 Railway 上的 API
CORS(app)

# --- 数据库配置 ---
# 在这里填入你的 MongoDB 连接字符串，替换 "你的连接字符串"
MONGO_URI = os.getenv("MONGO_URI", "mongodb+srv://ax9inl_db_user:<200412>@cluster0.d50pxnj.mongodb.net/?appName=Cluster0")
client = MongoClient(MONGO_URI, server_api=ServerApi('1'))
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
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0"
    }
    
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.encoding = 'utf-8'
        soup = BeautifulSoup(response.text, 'html.parser')

        fund_name = soup.find('span', class_='funCur-FundName')
        fund_name = fund_name.text.strip() if fund_name else "未找到基金名称"

        data_items = soup.find_all('dl', class_=lambda x: x and x.startswith('dataItem'))
        
        one_month_profit, one_year_profit, three_month_profit, six_month_profit = "N/A", "N/A", "N/A", "N/A"

        if len(data_items) > 0:
            dds = data_items[0].find_all('dd')
            if len(dds) > 1 and dds[1].find('span', class_='ui-num'):
                one_month_profit = dds[1].find('span', class_='ui-num').text.strip()
            if len(dds) > 2 and dds[2].find('span', class_='ui-num'):
                one_year_profit = dds[2].find('span', class_='ui-num').text.strip()
        if len(data_items) > 1:
            dds = data_items[1].find_all('dd')
            if len(dds) > 1 and dds[1].find('span', class_='ui-num'):
                three_month_profit = dds[1].find('span', class_='ui-num').text.strip()
        if len(data_items) > 2:
            dds = data_items[2].find_all('dd')
            if len(dds) > 1 and dds[1].find('span', class_='ui-num'):
                six_month_profit = dds[1].find('span', class_='ui-num').text.strip()

        return {
            "fund_code": fund_code,
            "fund_name": fund_name,
            "one_year_profit": one_year_profit,
            "six_month_profit": six_month_profit,
            "three_month_profit": three_month_profit,
            "one_month_profit": one_month_profit,
            "update_time": int(time.time())
        }
    except Exception as e:
        print(f"解析 {fund_code} 失败: {e}")
        return None

@app.route('/api/update', methods=['GET'])
def update_funds():
    """接口：触发爬虫并将数据写入 MongoDB"""
    code = request.args.get('code', 'all')
    codes_to_fetch = DEFAULT_FUND_CODES if code == 'all' else [code]
    
    updated_data = []
    for c in codes_to_fetch:
        data = get_fund_info(c)
        if data:
            # 使用 upsert 避免插入重复数据
            collection.update_one(
                {"fund_code": data["fund_code"]},
                {"$set": data},
                upsert=True
            )
            updated_data.append(data)
        time.sleep(0.5)
        
    return jsonify({"status": "success", "updated_count": len(updated_data)})

@app.route('/api/funds', methods=['GET'])
def get_funds():
    """接口：直接从 MongoDB 读取并返回所有数据给前端"""
    data = list(collection.find({}, {"_id": 0}))
    return jsonify(data)

if __name__ == "__main__":
    # 动态获取端口，Railway 部署刚需
    port = int(os.environ.get("PORT", 5000))
    app.run(host='0.0.0.0', port=port)