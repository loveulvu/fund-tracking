import os
import time
import requests
import traceback
import json
import re
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime, timedelta, timezone

app = Flask(__name__)
CORS(app, resources={r"/api/*": {"origins": "*"}}, supports_credentials=True)

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")
EMAIL_SENDER = os.environ.get("EMAIL_SENDER")
EMAIL_PASSWORD = os.environ.get("EMAIL_PASSWORD")

def send_verification_email(email, code):
    """
    使用 Resend HTTP API 发送验证码邮件
    - 返回 True/False
    """
    if not EMAIL_PASSWORD or not EMAIL_SENDER:
        print(f"[邮件] ⚠️ 邮件服务配置不完整: EMAIL_SENDER 或 EMAIL_PASSWORD 缺失")
        return False
    
    try:
        html_content = f"""
        <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
            <h2 style="color: #333;">验证码</h2>
            <p style="font-size: 16px; color: #666;">您的验证码是：</p>
            <div style="background: #f5f5f5; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 5px; margin: 20px 0;">
                {code}
            </div>
            <p style="font-size: 14px; color: #999;">验证码有效期为10分钟，请尽快使用。</p>
            <p style="font-size: 14px; color: #999;">如果您没有请求此验证码，请忽略此邮件。</p>
        </div>
        """
        
        response = requests.post(
            "https://api.resend.com/emails",
            headers={
                "Authorization": f"Bearer {EMAIL_PASSWORD}",
                "Content-Type": "application/json"
            },
            json={
                "from": EMAIL_SENDER,
                "to": [email],
                "subject": "您的验证码 - Fund Tracking",
                "html": html_content
            },
            timeout=10
        )
        
        if 200 <= response.status_code < 300:
            result = response.json()
            print(f"[邮件] ✅ 验证码已发送至 {email}, Resend ID: {result.get('id')}")
            return True
        else:
            print(f"[邮件] ❌ 发送失败: HTTP {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"[邮件] ❌ 彻底失败: {str(e)}")
        return False

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

SEED_FUNDS = [
    {"code": "008540", "name": "华夏科技创新A"},
    {"code": "012414", "name": "华夏中证新能源汽车ETF联接A"},
    {"code": "001887", "name": "华夏沪深300指数增强A"},
    {"code": "005303", "name": "华夏中证500指数增强A"},
    {"code": "588000", "name": "华夏上证科创板50成份ETF"},
    {"code": "161128", "name": "易方达中证海外中国互联网50ETF"},
    {"code": "510300", "name": "华泰柏瑞沪深300ETF"},
    {"code": "161725", "name": "招商中证白酒指数(LOF)A"},
    {"code": "001607", "name": "中欧医疗健康混合A"},
    {"code": "004243", "name": "易方达信息产业混合"}
]

DEFAULT_FUND_CODES = [fund["code"] for fund in SEED_FUNDS]

def validate_fund_data(fund_data, expected_code, expected_name_hint=None):
    """
    严格验证基金数据的有效性
    返回: (is_valid, reason)
    """
    if not fund_data:
        return False, "数据为空"
    
    fund_name = fund_data.get('fund_name', '')
    
    if fund_name == '未知' or not fund_name:
        return False, "基金名称为空或未知"
    
    if fund_data.get('fund_code') != expected_code:
        return False, f"基金代码不匹配: 期望 {expected_code}, 实际 {fund_data.get('fund_code')}"
    
    critical_fields = ['net_value', 'day_growth', 'week_growth', 'month_growth']
    missing_fields = [f for f in critical_fields if f not in fund_data]
    if missing_fields:
        return False, f"缺少关键字段: {missing_fields}"
    
    all_zero = all(
        fund_data.get(field, 0) == 0 
        for field in ['week_growth', 'month_growth', 'year_growth']
    )
    
    if all_zero and fund_name == '未知':
        return False, "所有涨幅为0且基金名称未知，数据可能无效"
    
    return True, "数据有效"

def init_seed_funds():
    """
    初始化种子基金，标记为 is_seed=True，并获取完整数据
    包含严格的数据验证，防止错误数据覆盖正确数据
    """
    if collection is None:
        print("[种子基金] 数据库未初始化，跳过种子基金初始化")
        return
    
    try:
        print("[种子基金] 开始清理旧的种子基金数据...")
        result = collection.delete_many({"is_seed": True})
        print(f"[种子基金] 已删除 {result.deleted_count} 条旧数据，准备重新初始化")
        
        initialized_count = 0
        failed_count = 0
        
        for seed in SEED_FUNDS:
            print(f"[种子基金] 正在获取 {seed['code']} ({seed['name']}) 的数据...")
            fund_data = get_fund_info(seed["code"])
            
            is_valid, reason = validate_fund_data(fund_data, seed["code"], seed["name"])
            
            if is_valid:
                fund_data["is_seed"] = True
                
                existing = collection.find_one({"fund_code": seed["code"]})
                
                if existing:
                    existing_valid, _ = validate_fund_data(existing, seed["code"])
                    
                    if not existing_valid:
                        collection.replace_one(
                            {"fund_code": seed["code"]},
                            fund_data
                        )
                        print(f"[种子基金] ✅ {seed['code']} 数据替换成功（旧数据无效）")
                    else:
                        collection.update_one(
                            {"fund_code": seed["code"]},
                            {"$set": {"is_seed": True}}
                        )
                        print(f"[种子基金] ✅ {seed['code']} 已有有效数据，仅更新标记")
                else:
                    collection.insert_one(fund_data)
                    print(f"[种子基金] ✅ {seed['code']} 数据插入成功: {fund_data.get('fund_name')}")
                
                initialized_count += 1
            else:
                failed_count += 1
                print(f"[种子基金] ❌ {seed['code']} 数据验证失败: {reason}")
                
                existing = collection.find_one({"fund_code": seed["code"]})
                if existing:
                    collection.update_one(
                        {"fund_code": seed["code"]},
                        {"$set": {"is_seed": True}}
                    )
                    print(f"[种子基金] ⚠️ {seed['code']} 保留现有数据，仅更新标记")
                else:
                    collection.insert_one({
                        "fund_code": seed["code"],
                        "fund_name": seed["name"],
                        "is_seed": True,
                        "update_time": int(time.time()),
                        "week_growth": 0.0,
                        "month_growth": 0.0,
                        "year_growth": 0.0,
                        "day_growth": 0.0,
                        "net_value": 0.0
                    })
                    print(f"[种子基金] ⚠️ {seed['code']} 使用基础数据模板")
            
            time.sleep(0.5)
        
        print(f"[种子基金] 初始化完成: 成功 {initialized_count} 个，失败 {failed_count} 个")
    except Exception as e:
        print(f"[种子基金] 初始化失败: {str(e)}")
        print(traceback.format_exc())

def get_fund_info(fund_code):
    """
    纯 API 驱动的基金信息获取函数
    数据源：
    1. fundgz.1234567.com.cn - 获取实时估值和日涨幅
    2. fundmobapi.eastmoney.com - 获取基金详情和历史涨幅
    """
    headers_web = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    headers_mobile = {
        'User-Agent': 'Dalvik/2.1.0 (Linux; U; Android 10; SM-G981B Build/QP1A.190711.020)',
        'Host': 'fundmobapi.eastmoney.com',
        'Connection': 'Keep-Alive',
    }
    
    print(f"[{fund_code}] 开始获取基金信息（纯API模式）...")
    
    data_item = {
        "fund_code": fund_code,
        "update_time": int(time.time())
    }
    
    try:
        api_url = f"https://fundgz.1234567.com.cn/js/{fund_code}.js"
        response = requests.get(api_url, headers=headers_web, timeout=5)
        response.encoding = 'utf-8'
        
        if response.status_code == 200 and 'jsonpgz' in response.text:
            json_str = response.text.replace('jsonpgz(', '').replace(');', '')
            fund_data = json.loads(json_str)
            
            data_item['fund_name'] = fund_data.get('name', '未知')
            data_item['net_value'] = float(fund_data.get('dwjz', 0)) if fund_data.get('dwjz') else 0.0
            data_item['net_value_date'] = fund_data.get('jzrq', '')
            data_item['day_growth'] = float(fund_data.get('gszzl', 0)) if fund_data.get('gszzl') else 0.0
            
            print(f"[{fund_code}] fundgz API: {data_item.get('fund_name')} | 净值: {data_item.get('net_value')} | 日涨幅: {data_item.get('day_growth')}%")
        else:
            print(f"[{fund_code}] fundgz API 返回异常: HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] fundgz API 获取失败: {str(e)}")
    
    try:
        base_info_url = f"http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo?FCODE={fund_code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0"
        response = requests.get(base_info_url, headers=headers_mobile, timeout=5)
        
        if response.status_code == 200:
            result = response.json()
            
            if result.get('Success') and result.get('Datas'):
                fund_info = result['Datas']
                
                if 'fund_name' not in data_item or data_item.get('fund_name') == '未知':
                    data_item['fund_name'] = fund_info.get('SHORTNAME', '未知')
                
                if 'net_value' not in data_item or data_item.get('net_value') == 0:
                    dwjz = fund_info.get('DWJZ')
                    if dwjz:
                        data_item['net_value'] = float(dwjz)
                
                data_item['fund_type'] = fund_info.get('FTYPE', '')
                data_item['fund_company'] = fund_info.get('JJGS', '')
                data_item['fund_manager'] = fund_info.get('JJJL', '')
                data_item['fund_scale'] = fund_info.get('TOTALSCALE', '')
                
                syl_z = fund_info.get('SYL_Z')
                syl_y = fund_info.get('SYL_Y')
                syl_3y = fund_info.get('SYL_3Y')
                syl_6y = fund_info.get('SYL_6Y')
                syl_1n = fund_info.get('SYL_1N')
                
                if syl_z is not None:
                    data_item['week_growth'] = float(syl_z)
                if syl_y is not None:
                    data_item['month_growth'] = float(syl_y)
                if syl_3y is not None:
                    data_item['three_month_growth'] = float(syl_3y)
                if syl_6y is not None:
                    data_item['six_month_growth'] = float(syl_6y)
                if syl_1n is not None:
                    data_item['year_growth'] = float(syl_1n)
                
                print(f"[{fund_code}] FundMNBaseInfo API: 周{data_item.get('week_growth', 'N/A')}% | 月{data_item.get('month_growth', 'N/A')}% | 年{data_item.get('year_growth', 'N/A')}%")
            else:
                print(f"[{fund_code}] FundMNBaseInfo API 无数据: {result.get('ErrMsg', 'Unknown error')}")
        else:
            print(f"[{fund_code}] FundMNBaseInfo API HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] FundMNBaseInfo API 获取失败: {str(e)}")
    
    growth_fields = ['week_growth', 'month_growth', 'three_month_growth', 'six_month_growth', 'year_growth', 'three_year_growth']
    for field in growth_fields:
        if field not in data_item:
            data_item[field] = 0.0
            print(f"[{fund_code}] {field} 缺失，使用默认值 0.0")
    
    if 'fund_name' not in data_item:
        data_item['fund_name'] = '未知'
    if 'net_value' not in data_item:
        data_item['net_value'] = 0.0
    if 'day_growth' not in data_item:
        data_item['day_growth'] = 0.0
    
    print(f"[{fund_code}] ✅ 数据获取完成: {data_item.get('fund_name')}")
    
    return data_item

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
                collection.replace_one({"fund_code": fund_code}, data, upsert=True)
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
            'addedAt': datetime.now(timezone.utc)
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
        
        success = send_verification_email(email, verification_code)
        
        if not success:
            print(f"[注册] ❌ 用户 {email} 邮件发送失败，拒绝注册")
            return jsonify({'error': '邮件服务通讯失败，请检查配置或稍后重试'}), 500
        
        expires_at = datetime.now(timezone.utc) + timedelta(minutes=10)
        
        pending_user = {
            'email': email,
            'password': password,
            'verification_code': verification_code,
            'verification_code_expires': expires_at,
            'createdAt': datetime.now(timezone.utc)
        }
        
        users_collection.insert_one(pending_user)
        
        print(f"[注册] ✅ 用户 {email} 注册成功，验证码已发送")
        return jsonify({'status': 'success', 'message': 'Verification code sent to your email'}), 200
        
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
            'exp': datetime.now(timezone.utc) + timedelta(hours=24)
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
