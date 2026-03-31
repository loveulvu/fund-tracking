import os
import time
import requests
from bs4 import BeautifulSoup
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime

app = Flask(__name__)
CORS(app)

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")

# 2. 全局数据库对象占位
db = None
collection = None
watchlist_collection = None
db_error_message = None

# 3. 柔性连接逻辑：即使失败也不触发进程崩溃，保证 Railway 能顺利启动
if not MONGO_URI:
    db_error_message = "CRITICAL: MONGO_URI 环境变量缺失，请在 Railway 的 Variables 中添加。"
    print(db_error_message)
else:
    try:
        # 设置 2 秒超时，防止网络卡死导致 Railway 健康检查超时
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=2000)
        db = client['fund_tracking']
        collection = db['fund_data']
        watchlist_collection = db['watchlists']
        # 测试连接
        client.admin.command('ping')
        print(f"Flask 正在连接数据库: {db.name}")
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
    
    print(f"[{fund_code}] 开始获取基金信息...")
    
    try:
        response = requests.get(url, headers=headers, timeout=3)
        response.encoding = 'utf-8'
        soup = BeautifulSoup(response.text, 'html.parser')
        
        fund_name = soup.find('span', class_='funCur-FundName')
        fund_name = fund_name.text.strip() if fund_name else "未知"
        
        data_item = {
            "fund_code": fund_code,
            "fund_name": fund_name,
            "update_time": int(time.time())
        }
        
        try:
            net_value_elem = soup.find('dl', class_='dataItem02')
            if net_value_elem:
                net_value = net_value_elem.find('span', class_='ui-font-large')
                if net_value:
                    data_item['net_value'] = float(net_value.text.strip())
                
                net_value_date = net_value_elem.find('dt')
                if net_value_date:
                    data_item['net_value_date'] = net_value_date.text.strip()
            
            day_growth_elem = soup.find('dl', class_='dataItem03')
            if day_growth_elem:
                day_growth = day_growth_elem.find('span', class_='ui-font-large')
                if day_growth:
                    growth_text = day_growth.text.strip().replace('%', '')
                    try:
                        data_item['day_growth'] = float(growth_text)
                    except:
                        pass
            
            data_items = soup.find_all('dl', class_='dataItem')
            for item in data_items:
                label = item.find('dt')
                value = item.find('dd')
                if label and value:
                    label_text = label.text.strip()
                    value_text = value.text.strip().replace('%', '')
                    
                    if '近1周' in label_text:
                        try:
                            data_item['week_growth'] = float(value_text)
                        except:
                            pass
                    elif '近1月' in label_text:
                        try:
                            data_item['month_growth'] = float(value_text)
                        except:
                            pass
                    elif '近3月' in label_text:
                        try:
                            data_item['three_month_growth'] = float(value_text)
                        except:
                            pass
                    elif '近6月' in label_text:
                        try:
                            data_item['six_month_growth'] = float(value_text)
                        except:
                            pass
                    elif '近1年' in label_text:
                        try:
                            data_item['year_growth'] = float(value_text)
                        except:
                            pass
                    elif '近3年' in label_text:
                        try:
                            data_item['three_year_growth'] = float(value_text)
                        except:
                            pass
        
        except Exception as e:
            print(f"[{fund_code}] 解析收益数据失败: {str(e)}")
        
        print(f"[{fund_code}] 获取成功: {fund_name}")
        return data_item
        
    except requests.exceptions.Timeout:
        print(f"[{fund_code}] ❌ 请求超时")
        return None
    except requests.exceptions.RequestException as e:
        print(f"[{fund_code}] ❌ 请求异常: {str(e)}")
        return None
    except Exception as e:
        print(f"[{fund_code}] ❌ 未知错误: {str(e)}")
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
    failed = []
    
    print("=" * 50)
    print(f"开始更新基金数据，共 {len(DEFAULT_FUND_CODES)} 个基金")
    print("=" * 50)
    
    for c in DEFAULT_FUND_CODES:
        data = get_fund_info(c)
        if data:
            try:
                collection.update_one({"fund_code": c}, {"$set": data}, upsert=True)
                updated.append(c)
                print(f"[{c}] ✅ 数据库更新成功")
            except Exception as e:
                failed.append({"code": c, "reason": f"数据库更新失败: {str(e)}"})
                print(f"[{c}] ❌ 数据库更新失败: {str(e)}")
        else:
            failed.append({"code": c, "reason": "获取基金信息失败"})
        
        # 添加短暂延迟，避免请求过快
        time.sleep(0.5)
    
    print("=" * 50)
    print(f"更新完成: 成功 {len(updated)} 个，失败 {len(failed)} 个")
    print("=" * 50)
    
    return jsonify({
        "status": "success",
        "count": len(updated),
        "codes": updated,
        "failed": failed,
        "total": len(DEFAULT_FUND_CODES)
    })

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        data = list(collection.find({}, {"_id": 0}))
        return jsonify(data)
    except Exception as e:
        print(f"获取基金列表失败: {str(e)}")
        return jsonify({"status": "error", "message": f"获取数据失败: {str(e)}"}), 500

@app.route('/api/fund/<fund_code>')
def get_fund(fund_code):
    """获取单个基金数据"""
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        # 先从数据库查找
        fund_data = collection.find_one({"fund_code": fund_code}, {"_id": 0})
        
        if fund_data:
            print(f"[{fund_code}] 从数据库获取成功")
            return jsonify(fund_data)
        
        # 数据库中没有，从天天基金网爬取
        print(f"[{fund_code}] 数据库中不存在，从天天基金网获取...")
        data = get_fund_info(fund_code)
        
        if data:
            # 保存到数据库
            collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
            print(f"[{fund_code}] ✅ 从天天基金网获取成功并保存到数据库")
            return jsonify(data)
        else:
            return jsonify({"error": "Fund not found"}), 404
            
    except Exception as e:
        print(f"获取基金数据失败: {str(e)}")
        return jsonify({"error": f"Failed to fetch fund data: {str(e)}"}), 500

@app.route('/')
def index():
    if db_error_message:
        return f"API is Running, but Database Error: {db_error_message}", 200
    return "Fund Tracking API is Running successfully with DB connected.", 200

@app.route('/health')
def health():
    return jsonify({
        "status": "ok",
        "db_connected": collection is not None,
        "db_error": db_error_message
    })

# ============ JWT 验证装饰器 ============

def token_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        token = None
        
        # 从请求头获取 token
        if 'Authorization' in request.headers:
            auth_header = request.headers['Authorization']
            if auth_header.startswith('Bearer '):
                token = auth_header.split(' ')[1]
        
        if not token:
            return jsonify({'error': 'Token is missing'}), 401
        
        try:
            # 验证 token
            data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
            current_user_id = data['userId']
        except jwt.ExpiredSignatureError:
            return jsonify({'error': 'Token has expired'}), 401
        except jwt.InvalidTokenError:
            return jsonify({'error': 'Invalid token'}), 401
        
        return f(current_user_id, *args, **kwargs)
    
    return decorated

# ============ 关注列表 API ============

print("System: Loading Route /api/watchlist...")

@app.route('/api/watchlist', methods=['GET', 'POST', 'OPTIONS'])
def get_watchlist():
    """获取关注列表（暂时返回所有数据，稍后恢复鉴权）"""
    print(f"[DEBUG] Received request: {request.method} {request.path}")
    
    # 处理 OPTIONS 预检请求
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        # 暂时返回所有关注列表数据，不按用户过滤
        watchlist = list(watchlist_collection.find({}, {'_id': 0}))
        print(f"[DEBUG] Returning {len(watchlist)} items")
        return jsonify(watchlist)
    except Exception as e:
        print(f"获取关注列表失败: {str(e)}")
        return jsonify({'error': 'Failed to fetch watchlist'}), 500

@app.route('/api/watchlist', methods=['POST'])
@token_required
def add_to_watchlist(current_user_id):
    """添加基金到关注列表"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        fund_code = data.get('fundCode')
        fund_name = data.get('fundName')
        threshold = data.get('alertThreshold', 5)
        
        if not fund_code or not fund_name:
            return jsonify({'error': 'fundCode and fundName are required'}), 400
        
        # 检查是否已存在
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if existing:
            return jsonify({'error': 'Fund already in watchlist'}), 400
        
        # 添加到关注列表
        watchlist_item = {
            'userId': current_user_id,
            'fundCode': fund_code,
            'fundName': fund_name,
            'alertThreshold': threshold,
            'addedAt': datetime.utcnow()
        }
        
        watchlist_collection.insert_one(watchlist_item)
        
        # 返回时移除 _id
        watchlist_item.pop('_id', None)
        return jsonify(watchlist_item), 201
        
    except Exception as e:
        print(f"添加关注失败: {str(e)}")
        return jsonify({'error': 'Failed to add to watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['DELETE'])
@token_required
def remove_from_watchlist(current_user_id, fund_code):
    """从关注列表中删除基金"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        result = watchlist_collection.delete_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if result.deleted_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        return jsonify({'message': 'Successfully removed from watchlist'})
        
    except Exception as e:
        print(f"删除关注失败: {str(e)}")
        return jsonify({'error': 'Failed to remove from watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['PUT'])
@token_required
def update_watchlist_threshold(current_user_id, fund_code):
    """更新关注基金的预警阈值"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        new_threshold = data.get('alertThreshold')
        
        if new_threshold is None:
            return jsonify({'error': 'alertThreshold is required'}), 400
        
        result = watchlist_collection.update_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'$set': {'alertThreshold': new_threshold}}
        )
        
        if result.matched_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        # 返回更新后的数据
        updated_item = watchlist_collection.find_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'_id': 0}
        )
        
        return jsonify(updated_item)
        
    except Exception as e:
        print(f"更新阈值失败: {str(e)}")
        return jsonify({'error': 'Failed to update threshold'}), 500

if __name__ == "__main__":
    # Railway 必须动态读取 PORT 变量
    port = int(os.environ.get("PORT", 8080))
    print(f"Starting Flask server on port {port}...")
    app.run(host='0.0.0.0', port=port)
