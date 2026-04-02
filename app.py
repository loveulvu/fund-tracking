import os
import time
import requests
import traceback
from bs4 import BeautifulSoup
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime, timedelta

app = Flask(__name__)
CORS(app, resources={r"/api/*": {"origins": "*"}}, supports_credentials=True)

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")

# 2. 全局数据库对象占位
db = None
collection = None
watchlist_collection = None
users_collection = None
db_error_message = None

# 3. 柔性连接逻辑：即使失败也不触发进程崩溃，保证 Railway 能顺利启动
if not MONGO_URI:
    db_error_message = "CRITICAL: MONGO_URI 环境变量缺失，请在 Railway 的 Variables 中添加。"
    print(db_error_message)
else:
    try:
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=5000)
        db = client['fund_tracking']
        collection = db['fund_data']
        watchlist_collection = db['watchlists']
        users_collection = db['users']
        client.admin.command('ping')
        print(f"Flask 正在连接数据库: {db.name}")
        print("MongoDB 连接成功。")
    except Exception as e:
        db_error_message = f"MongoDB 连接失败: {str(e)}"
        print(db_error_message)
        print("完整错误堆栈:")
        print(traceback.format_exc())

DEFAULT_FUND_CODES = [
    "000001", "006030", "110022", "161725", "110011",
    "008540", "510300", "510500", "159915", "159919",
    "000011", "000031", "000041", "000051", "000061",
    "000071", "000081", "000091", "000101", "000111"
]

SEED_FUNDS = [
    {"code": "000001", "name": "华夏成长"},
    {"code": "006030", "name": "华夏中证500ETF联接A"},
    {"code": "110022", "name": "易方达消费行业"},
    {"code": "161725", "name": "招商中证白酒指数"},
    {"code": "110011", "name": "易方达中小盘"},
    {"code": "008540", "name": "华夏科技创新A"},
    {"code": "510300", "name": "华泰柏瑞沪深300ETF"},
    {"code": "510500", "name": "华夏中证500ETF"},
    {"code": "159915", "name": "易方达创业板ETF"},
    {"code": "159919", "name": "嘉实沪深300ETF"},
    {"code": "000011", "name": "华夏大盘精选"},
    {"code": "000031", "name": "华夏复兴"},
    {"code": "000041", "name": "华夏优势增长"},
    {"code": "000051", "name": "华夏回报"},
    {"code": "000061", "name": "华夏回报二号"},
    {"code": "000071", "name": "华夏红利"},
    {"code": "000081", "name": "华夏策略精选"},
    {"code": "000091", "name": "华夏平稳增长"},
    {"code": "000101", "name": "华夏蓝筹核心"},
    {"code": "000111", "name": "华夏经典配置"}
]

def init_seed_funds():
    """
    初始化种子基金，标记为 is_seed=True，并获取完整数据
    """
    if collection is None:
        print("[种子基金] 数据库未初始化，跳过种子基金初始化")
        return
    
    try:
        initialized_count = 0
        updated_count = 0
        refreshed_count = 0
        
        for seed in SEED_FUNDS:
            existing = collection.find_one({"fund_code": seed["code"]})
            
            if existing:
                needs_update = False
                
                if 'net_value' not in existing or existing.get('net_value') is None:
                    needs_update = True
                    print(f"[种子基金] {seed['code']} 缺少净值数据，需要更新")
                
                if 'day_growth' not in existing or existing.get('day_growth') is None:
                    needs_update = True
                    print(f"[种子基金] {seed['code']} 缺少日涨幅数据，需要更新")
                
                if 'month_growth' not in existing or existing.get('month_growth') is None:
                    needs_update = True
                    print(f"[种子基金] {seed['code']} 缺少月收益数据，需要更新")
                
                if needs_update:
                    print(f"[种子基金] 正在更新 {seed['code']} ({seed['name']}) 的完整数据...")
                    fund_data = get_fund_info(seed["code"])
                    
                    if fund_data:
                        fund_data["is_seed"] = True
                        collection.update_one(
                            {"fund_code": seed["code"]},
                            {"$set": fund_data}
                        )
                        refreshed_count += 1
                        print(f"[种子基金] ✅ {seed['code']} 数据更新成功")
                    else:
                        collection.update_one(
                            {"fund_code": seed["code"]},
                            {"$set": {"is_seed": True}}
                        )
                        print(f"[种子基金] ⚠️ {seed['code']} 数据更新失败，仅更新标记")
                    
                    time.sleep(0.5)
                else:
                    collection.update_one(
                        {"fund_code": seed["code"]},
                        {"$set": {"is_seed": True}}
                    )
                    updated_count += 1
            else:
                print(f"[种子基金] 正在获取 {seed['code']} ({seed['name']}) 的完整数据...")
                fund_data = get_fund_info(seed["code"])
                
                if fund_data:
                    fund_data["is_seed"] = True
                    collection.insert_one(fund_data)
                    initialized_count += 1
                    print(f"[种子基金] ✅ {seed['code']} 数据获取成功")
                else:
                    collection.insert_one({
                        "fund_code": seed["code"],
                        "fund_name": seed["name"],
                        "is_seed": True,
                        "update_time": int(time.time())
                    })
                    initialized_count += 1
                    print(f"[种子基金] ⚠️ {seed['code']} 使用基础数据")
                
                time.sleep(0.5)
        
        print(f"[种子基金] 初始化完成: 新增 {initialized_count} 个，更新 {updated_count} 个，刷新 {refreshed_count} 个")
    except Exception as e:
        print(f"[种子基金] 初始化失败: {str(e)}")

def get_fund_info(fund_code):
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    
    print(f"[{fund_code}] 开始获取基金信息...")
    
    try:
        data_item = {
            "fund_code": fund_code,
            "update_time": int(time.time())
        }
        
        try:
            api_url = f"http://fundgz.1234567.com.cn/js/{fund_code}.js"
            response = requests.get(api_url, headers=headers, timeout=3)
            response.encoding = 'utf-8'
            
            jsonp_str = response.text
            if jsonp_str and 'jsonpgz' in jsonp_str:
                json_str = jsonp_str.replace('jsonpgz(', '').replace(');', '')
                import json
                fund_data = json.loads(json_str)
                
                data_item['fund_name'] = fund_data.get('name', '未知')
                data_item['net_value'] = float(fund_data.get('dwjz', 0))
                data_item['net_value_date'] = fund_data.get('jzrq', '')
                data_item['day_growth'] = float(fund_data.get('gszzl', 0))
                
                print(f"[{fund_code}] 从 API 获取基本信息成功: {data_item['fund_name']}")
        except Exception as e:
            print(f"[{fund_code}] API 获取失败: {str(e)}")
        
        try:
            url = f"https://fund.eastmoney.com/{fund_code}.html"
            response = requests.get(url, headers=headers, timeout=3)
            response.encoding = 'utf-8'
            soup = BeautifulSoup(response.text, 'html.parser')
            
            if 'fund_name' not in data_item:
                fund_name = soup.find('span', class_='funCur-FundName')
                data_item['fund_name'] = fund_name.text.strip() if fund_name else "未知"
            
            if 'net_value' not in data_item:
                net_value_elem = soup.find('dl', class_='dataItem02')
                if net_value_elem:
                    net_value = net_value_elem.find('span', class_='ui-font-large')
                    if net_value:
                        data_item['net_value'] = float(net_value.text.strip())
                    
                    net_value_date = net_value_elem.find('dt')
                    if net_value_date:
                        data_item['net_value_date'] = net_value_date.text.strip()
            
            if 'day_growth' not in data_item:
                day_growth_elem = soup.find('dl', class_='dataItem03')
                if day_growth_elem:
                    day_growth = day_growth_elem.find('span', class_='ui-font-large')
                    if day_growth:
                        growth_text = day_growth.text.strip().replace('%', '')
                        try:
                            data_item['day_growth'] = float(growth_text)
                        except:
                            pass
            
            data_items = soup.find_all('div', class_='dataOfFund')
            print(f"[{fund_code}] 找到 {len(data_items)} 个数据区域")
            
            for item in data_items:
                labels = item.find_all('label')
                for label in labels:
                    text = label.text.strip()
                    if '近1周' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['week_growth'] = float(value)
                            print(f"[{fund_code}] 近1周: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近1周解析失败: {str(e)}")
                    elif '近1月' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['month_growth'] = float(value)
                            print(f"[{fund_code}] 近1月: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近1月解析失败: {str(e)}")
                    elif '近3月' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['three_month_growth'] = float(value)
                            print(f"[{fund_code}] 近3月: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近3月解析失败: {str(e)}")
                    elif '近6月' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['six_month_growth'] = float(value)
                            print(f"[{fund_code}] 近6月: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近6月解析失败: {str(e)}")
                    elif '近1年' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['year_growth'] = float(value)
                            print(f"[{fund_code}] 近1年: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近1年解析失败: {str(e)}")
                    elif '近3年' in text:
                        try:
                            value = label.find_next('span').text.strip().replace('%', '')
                            data_item['three_year_growth'] = float(value)
                            print(f"[{fund_code}] 近3年: {value}%")
                        except Exception as e:
                            print(f"[{fund_code}] 近3年解析失败: {str(e)}")
            
            if 'week_growth' not in data_item:
                data_item['week_growth'] = 0
            if 'month_growth' not in data_item:
                data_item['month_growth'] = 0
            if 'three_month_growth' not in data_item:
                data_item['three_month_growth'] = 0
            if 'six_month_growth' not in data_item:
                data_item['six_month_growth'] = 0
            if 'year_growth' not in data_item:
                data_item['year_growth'] = 0
            if 'three_year_growth' not in data_item:
                data_item['three_year_growth'] = 0
            
            print(f"[{fund_code}] 从主页获取收益数据成功")
        except Exception as e:
            print(f"[{fund_code}] 主页获取失败: {str(e)}")
        
        print(f"[{fund_code}] 获取完成: {data_item.get('fund_name', '未知')}")
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

init_seed_funds()

def check_db_status():
    if db_error_message:
        return jsonify({"status": "error", "message": db_error_message}), 500
    if collection is None:
        return jsonify({"status": "error", "message": "数据库未初始化"}), 500
    return None

def token_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        if request.method == 'OPTIONS':
            return jsonify({'status': 'ok'}), 200
        
        token = None
        
        if 'Authorization' in request.headers:
            auth_header = request.headers['Authorization']
            if auth_header.startswith('Bearer '):
                token = auth_header.split(' ')[1]
        
        if not token:
            return jsonify({'error': 'Token is missing'}), 401
        
        try:
            data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
            current_user_id = data['userId']
        except jwt.ExpiredSignatureError:
            return jsonify({'error': 'Token has expired'}), 401
        except jwt.InvalidTokenError:
            return jsonify({'error': 'Invalid token'}), 401
        
        return f(current_user_id, *args, **kwargs)
    
    return decorated

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

@app.route('/api/update_seeds')
def update_seed_funds():
    """
    更新所有种子基金的完整数据
    """
    db_check = check_db_status()
    if db_check: return db_check
    
    updated = []
    failed = []
    
    print("=" * 50)
    print(f"开始更新种子基金数据，共 {len(SEED_FUNDS)} 个")
    print("=" * 50)
    
    for seed in SEED_FUNDS:
        fund_code = seed["code"]
        print(f"[{fund_code}] 正在获取完整数据...")
        
        data = get_fund_info(fund_code)
        if data:
            try:
                data["is_seed"] = True
                collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
                updated.append(fund_code)
                print(f"[{fund_code}] ✅ 数据库更新成功")
            except Exception as e:
                failed.append({"code": fund_code, "reason": f"数据库更新失败: {str(e)}"})
                print(f"[{fund_code}] ❌ 数据库更新失败: {str(e)}")
        else:
            failed.append({"code": fund_code, "reason": "获取基金信息失败"})
        
        time.sleep(0.5)
    
    print("=" * 50)
    print(f"更新完成: 成功 {len(updated)} 个，失败 {len(failed)} 个")
    print("=" * 50)
    
    return jsonify({
        "status": "success",
        "count": len(updated),
        "codes": updated,
        "failed": failed,
        "total": len(SEED_FUNDS)
    })

@app.route('/api/search_proxy')
def search_proxy():
    """
    搜索代理接口：调用天天基金网的搜索API
    """
    query = request.args.get('query', '')
    
    if not query:
        return jsonify({"error": "Query parameter is required"}), 400
    
    try:
        headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
        
        search_url = f"http://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx?m=1&key={query}"
        
        response = requests.get(search_url, headers=headers, timeout=5)
        response.encoding = 'utf-8'
        
        if response.status_code == 200:
            import json
            data = json.loads(response.text)
            
            if 'Datas' in data:
                results = []
                for item in data['Datas'][:20]:
                    results.append({
                        'fund_code': item.get('CODE', ''),
                        'fund_name': item.get('NAME', ''),
                        'fund_type': item.get('FUNDTYPE', ''),
                        'fund_family': item.get('FUNDNAME', '')
                    })
                
                print(f"[搜索代理] 查询 '{query}' 找到 {len(results)} 个结果")
                return jsonify(results)
            else:
                return jsonify([])
        else:
            return jsonify({"error": "Failed to fetch search results"}), 500
            
    except requests.exceptions.Timeout:
        print(f"[搜索代理] 查询 '{query}' 超时")
        return jsonify({"error": "Search request timeout"}), 504
    except Exception as e:
        print(f"[搜索代理] 查询 '{query}' 失败: {str(e)}")
        return jsonify({"error": f"Search failed: {str(e)}"}), 500

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        token = None
        if 'Authorization' in request.headers:
            auth_header = request.headers['Authorization']
            if auth_header.startswith('Bearer '):
                token = auth_header.split(' ')[1]
        
        watched_fund_codes = []
        if token:
            try:
                data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
                current_user_id = data['userId']
                watched_items = list(watchlist_collection.find({'userId': current_user_id}, {'fundCode': 1, '_id': 0}))
                watched_fund_codes = [item['fundCode'] for item in watched_items]
            except:
                pass
        
        all_funds = list(collection.find({}, {"_id": 0}))
        
        for fund in all_funds:
            if fund.get('fund_code') in watched_fund_codes:
                fund['is_watched'] = True
            else:
                fund['is_watched'] = False
        
        sorted_funds = sorted(all_funds, key=lambda x: (
            not x.get('is_watched', False),
            not x.get('is_seed', False),
            x.get('fund_code', '')
        ))
        
        print(f"[基金列表] 返回 {len(sorted_funds)} 个基金，其中 {len(watched_fund_codes)} 个已关注")
        return jsonify(sorted_funds)
    except Exception as e:
        print(f"获取基金列表失败: {str(e)}")
        return jsonify({"status": "error", "message": f"获取数据失败: {str(e)}"}), 500

@app.route('/api/fund/<fund_code>')
def get_fund(fund_code):
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        fund_data = collection.find_one({"fund_code": fund_code}, {"_id": 0})
        
        if fund_data:
            print(f"[{fund_code}] 从数据库获取成功")
            return jsonify(fund_data)
        
        print(f"[{fund_code}] 数据库中不存在，从天天基金网获取...")
        data = get_fund_info(fund_code)
        
        if data:
            collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
            print(f"[{fund_code}] ✅ 从天天基金网获取成功并保存到数据库")
            return jsonify(data)
        else:
            return jsonify({"error": "Fund not found"}), 404
            
    except Exception as e:
        print(f"获取基金数据失败: {str(e)}")
        return jsonify({"error": f"Failed to fetch fund data: {str(e)}"}), 500

@app.route('/api/watchlist', methods=['GET', 'OPTIONS'])
def get_watchlist():
    print(f"[DEBUG] get_watchlist 被调用，方法: {request.method}")
    if request.method == 'OPTIONS':
        print("[DEBUG] OPTIONS 请求，直接返回 200")
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] 数据库检查失败: {db_check}")
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        print(f"[DEBUG] Authorization header: {auth_header[:20]}..." if len(auth_header) > 20 else f"[DEBUG] Authorization header: {auth_header}")
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        print("[DEBUG] Token 缺失")
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
        print(f"[DEBUG] Token 解码成功，用户ID: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token 已过期")
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError as e:
        print(f"[DEBUG] Token 无效: {str(e)}")
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        watchlist = list(watchlist_collection.find({'userId': current_user_id}, {'_id': 0}))
        print(f"[关注列表] 用户 {current_user_id} 共 {len(watchlist)} 条记录")
        return jsonify(watchlist)
    except Exception as e:
        print(f"获取关注列表失败: {str(e)}")
        return jsonify({'error': 'Failed to fetch watchlist'}), 500

@app.route('/api/watchlist', methods=['POST', 'OPTIONS'])
def add_to_watchlist():
    print(f"[DEBUG] add_to_watchlist 被调用，方法: {request.method}")
    if request.method == 'OPTIONS':
        print("[DEBUG] OPTIONS 请求，直接返回 200")
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] 数据库检查失败: {db_check}")
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        print(f"[DEBUG] Authorization header: {auth_header[:20]}..." if len(auth_header) > 20 else f"[DEBUG] Authorization header: {auth_header}")
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        print("[DEBUG] Token 缺失")
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
        print(f"[DEBUG] Token 解码成功，用户ID: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token 已过期")
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError as e:
        print(f"[DEBUG] Token 无效: {str(e)}")
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        req_data = request.get_json()
        print(f"[DEBUG] 请求数据: {req_data}")
        fund_code = req_data.get('fundCode')
        fund_name = req_data.get('fundName')
        threshold = req_data.get('alertThreshold', 5)
        
        if not fund_code or not fund_name:
            print(f"[DEBUG] 缺少必要参数: fundCode={fund_code}, fundName={fund_name}")
            return jsonify({'error': 'fundCode and fundName are required'}), 400
        
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if existing:
            print(f"[DEBUG] 基金 {fund_code} 已在关注列表中")
            return jsonify({'error': 'Fund already in watchlist'}), 400
        
        watchlist_item = {
            'userId': current_user_id,
            'fundCode': fund_code,
            'fundName': fund_name,
            'alertThreshold': threshold,
            'addedAt': datetime.utcnow()
        }
        
        watchlist_collection.insert_one(watchlist_item)
        watchlist_item.pop('_id', None)
        print(f"[添加关注] 用户 {current_user_id} 添加 {fund_code}")
        return jsonify(watchlist_item), 201
        
    except Exception as e:
        print(f"添加关注失败: {str(e)}")
        return jsonify({'error': 'Failed to add to watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['DELETE', 'OPTIONS'])
def remove_from_watchlist(fund_code):
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
    except jwt.ExpiredSignatureError:
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError:
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        result = watchlist_collection.delete_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if result.deleted_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        print(f"[删除关注] 用户 {current_user_id} 删除 {fund_code}")
        return jsonify({'message': 'Successfully removed from watchlist'})
        
    except Exception as e:
        print(f"删除关注失败: {str(e)}")
        return jsonify({'error': 'Failed to remove from watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['PUT', 'OPTIONS'])
def update_watchlist_threshold(fund_code):
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
    except jwt.ExpiredSignatureError:
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError:
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        req_data = request.get_json()
        new_threshold = req_data.get('alertThreshold')
        
        if new_threshold is None:
            return jsonify({'error': 'alertThreshold is required'}), 400
        
        result = watchlist_collection.update_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'$set': {'alertThreshold': new_threshold}}
        )
        
        if result.matched_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        updated_item = watchlist_collection.find_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'_id': 0}
        )
        
        print(f"[更新阈值] 用户 {current_user_id} 更新 {fund_code} 阈值为 {new_threshold}")
        return jsonify(updated_item)
        
    except Exception as e:
        print(f"更新阈值失败: {str(e)}")
        return jsonify({'error': 'Failed to update threshold'}), 500

@app.route('/api/auth/register', methods=['POST', 'OPTIONS'])
def register():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        existing_user = users_collection.find_one({'email': email})
        if existing_user:
            return jsonify({'error': 'User already exists'}), 409
        
        import random
        verification_code = str(random.randint(100000, 999999))
        expires_at = datetime.utcnow() + timedelta(minutes=10)
        
        pending_user = {
            'email': email,
            'password': password,
            'verification_code': verification_code,
            'verification_code_expires': expires_at,
            'createdAt': datetime.utcnow()
        }
        
        users_collection.insert_one(pending_user)
        print(f"[注册] 用户 {email} 注册成功，验证码: {verification_code}")
        
        return jsonify({'status': 'success', 'message': 'User registered. Verification code: ' + verification_code}), 200
        
    except Exception as e:
        print(f"[注册] ❌ 注册失败: {str(e)}")
        return jsonify({'error': 'Registration failed'}), 500

@app.route('/api/auth/login', methods=['POST', 'OPTIONS'])
def login():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        user = users_collection.find_one({'email': email, 'password': password})
        
        if not user:
            return jsonify({'error': 'Invalid email or password'}), 401
        
        payload = {
            'userId': str(user['_id']),
            'email': email,
            'exp': datetime.utcnow() + timedelta(hours=24)
        }
        
        token = jwt.encode(payload, JWT_SECRET, algorithm='HS256')
        
        print(f"[登录] 用户 {email} 登录成功")
        return jsonify({
            'status': 'success',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"[登录] ❌ 登录失败: {str(e)}")
        return jsonify({'error': 'Login failed'}), 500

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

@app.route('/api/health')
def api_health():
    return jsonify({"status": "ok"})

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    print(f"Starting Flask server on port {port}...")
    app.run(host='0.0.0.0', port=port)
