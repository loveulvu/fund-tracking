import os
import time
import requests
import random
import threading
from bs4 import BeautifulSoup
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime, timedelta
import resend

app = Flask(__name__)
CORS(app, resources={r"/api/*": {"origins": "*"}})

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")
RESEND_API_KEY = os.environ.get("RESEND_API_KEY")

# 初始化 Resend
if RESEND_API_KEY:
    resend.api_key = RESEND_API_KEY

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
        # 设置 2 秒超时，防止网络卡死导致 Railway 健康检查超时
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=2000)
        # 兼容两种连接串格式：带/不带数据库名称
        db = client['fund_tracking']
        collection = db['fund_data']
        watchlist_collection = db['watchlists']
        users_collection = db['users']
        # 测试连接
        client.admin.command('ping')
        print(f"Flask 正在连接数据库: {db.name}")
        print("MongoDB 连接成功。")
        
        # 清理脏数据：删除所有不属于已注册用户的关注列表记录
        try:
            all_user_ids = [str(u['_id']) for u in users_collection.find({}, {'_id': 1})]
            result = watchlist_collection.delete_many({
                'userId': {'$nin': all_user_ids}
            })
            if result.deleted_count > 0:
                print(f"[清理] 已删除 {result.deleted_count} 条脏数据（不属于任何用户的关注记录）")
        except Exception as e:
            print(f"[清理] 清理脏数据失败: {str(e)}")
    except Exception as e:
        db_error_message = f"MongoDB 连接失败: {str(e)}"
        print(db_error_message)

DEFAULT_FUND_CODES = [
    "006030", "000001", "110022", "161725", "110011"
]

def get_fund_info(fund_code):
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    
    print(f"[{fund_code}] 开始获取基金信息...")
    
    try:
        url = f"http://fund.eastmoney.com/{fund_code}.html"
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        response.encoding = 'utf-8'
        soup = BeautifulSoup(response.text, 'html.parser')
        
        fund_name = soup.find('div', class_='box').find('span').text.strip() if soup.find('div', class_='box') else None
        if not fund_name:
            fund_name = soup.find('div', {'id': 'infoName'}).find('span').text.strip() if soup.find('div', {'id': 'infoName'}) else None
        
        if not fund_name:
            print(f"[{fund_code}] ❌ 基金名称获取失败")
            return None
        
        print(f"[{fund_code}] ✅ 基金名称: {fund_name}")
        
        data = {
            'fund_code': fund_code,
            'fund_name': fund_name,
            'update_time': int(time.time())
        }
        
        net_worth = soup.find('span', class_='ui-font-large') or soup.find('span', {'id': 'gz_gsz'})
        if net_worth:
            data['net_worth'] = float(net_worth.text.strip())
            print(f"[{fund_code}] ✅ 净值: {data['net_worth']}")
        
        day_growth = soup.find('span', {'id': 'gz_gsz'}) or soup.find('span', class_='ui-font-large')
        if day_growth and day_growth != net_worth:
            growth_text = day_growth.text.strip()
            if growth_text:
                data['day_growth'] = float(growth_text.replace('%', ''))
                print(f"[{fund_code}] ✅ 日涨幅: {data['day_growth']}%")
        
        today_purchase = soup.find('span', {'id': 'gz_gsz'})
        if today_purchase:
            data['today_purchase'] = today_purchase.text.strip()
        
        fund_type = soup.find('div', class_='infoOfFund')
        if fund_type:
            type_rows = fund_type.find_all('tr')
            for row in type_rows:
                cells = row.find_all('td')
                if len(cells) >= 2:
                    key = cells[0].text.strip().replace('：', '')
                    value = cells[1].text.strip()
                    if '基金类型' in key:
                        data['fund_type'] = value
                    elif '基金规模' in key:
                        data['fund_size'] = value
        
        data['week_growth'] = 0.0
        data['month_growth'] = 0.0
        data['three_month_growth'] = 0.0
        data['six_month_growth'] = 0.0
        data['year_growth'] = 0.0
        data['two_year_growth'] = 0.0
        data['three_year_growth'] = 0.0
        data['five_year_growth'] = 0.0
        data['this_year_growth'] = 0.0
        data['total_growth'] = 0.0
        
        history_table = soup.find('table', class_='ui-table ui-table-border ui-table-hover ui-table-radius')
        if history_table:
            rows = history_table.find_all('tr')
            for row in rows[1:]:
                cells = row.find_all('td')
                if len(cells) >= 2:
                    period = cells[0].text.strip()
                    growth = cells[1].text.strip().replace('%', '') if cells[1].text.strip() else '0.0'
                    try:
                        growth_value = float(growth)
                    except ValueError:
                        growth_value = 0.0
                    
                    if '近1周' in period:
                        data['week_growth'] = growth_value
                    elif '近1月' in period:
                        data['month_growth'] = growth_value
                    elif '近3月' in period:
                        data['three_month_growth'] = growth_value
                    elif '近6月' in period:
                        data['six_month_growth'] = growth_value
                    elif '近1年' in period:
                        data['year_growth'] = growth_value
                    elif '近2年' in period:
                        data['two_year_growth'] = growth_value
                    elif '近3年' in period:
                        data['three_year_growth'] = growth_value
                    elif '近5年' in period:
                        data['five_year_growth'] = growth_value
                    elif '今年来' in period:
                        data['this_year_growth'] = growth_value
                    elif '成立以来' in period:
                        data['total_growth'] = growth_value
        
        print(f"[{fund_code}] ✅ 历史收益数据已提取")
        
        return data
        
    except requests.exceptions.RequestException as e:
        print(f"[{fund_code}] ❌ 请求失败: {str(e)}")
        return None
    except Exception as e:
        print(f"[{fund_code}] ❌ 解析失败: {str(e)}")
        return None

def update_fund_data():
    print("[更新] 开始更新基金数据...")
    
    for fund_code in DEFAULT_FUND_CODES:
        try:
            fund_data = get_fund_info(fund_code)
            if fund_data:
                collection.update_one(
                    {'fund_code': fund_code},
                    {'$set': fund_data},
                    upsert=True
                )
                print(f"[{fund_code}] ✅ 数据已更新")
            else:
                print(f"[{fund_code}] ❌ 数据获取失败")
        except Exception as e:
            print(f"[{fund_code}] ❌ 更新失败: {str(e)}")
    
    print("[更新] 基金数据更新完成。")

@app.route('/api/update')
def update_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    threading.Thread(target=update_fund_data).start()
    return jsonify({"message": "更新任务已启动"}), 202

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        funds = list(collection.find({}, {'_id': 0}))
        return jsonify(funds)
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/fund/<fund_code>')
def get_fund(fund_code):
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        fund = collection.find_one({'fund_code': fund_code}, {'_id': 0})
        if fund:
            return jsonify(fund)
        else:
            return jsonify({"error": "Fund not found"}), 404
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/funds/search')
def search_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        query = request.args.get('query', '').strip()
        
        if not query:
            return jsonify({"error": "Query parameter is required"}), 400
        
        fund = collection.find_one({'fund_code': query}, {'_id': 0})
        if fund:
            return jsonify([fund])
        
        fund = collection.find_one({'fund_name': {'$regex': query, '$options': 'i'}}, {'_id': 0})
        if fund:
            return jsonify([fund])
        
        new_fund = get_fund_info(query)
        if new_fund:
            collection.update_one(
                {'fund_code': query},
                {'$set': new_fund},
                upsert=True
            )
            return jsonify([new_fund])
        
        return jsonify([]), 404
    except Exception as e:
        return jsonify({"error": str(e)}), 500

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

@app.route('/api/watchlist', methods=['GET'])
@token_required
def get_watchlist(current_user_id):
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        watchlist = list(watchlist_collection.find({'userId': current_user_id}, {'_id': 0}))
        return jsonify(watchlist)
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/watchlist', methods=['POST'])
@token_required
def add_to_watchlist(current_user_id):
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        fund_code = data.get('fundCode')
        fund_name = data.get('fundName')
        threshold = data.get('alertThreshold', 0)
        
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if existing:
            return jsonify({'error': 'Fund already in watchlist'}), 409
        
        watchlist_collection.insert_one({
            'userId': current_user_id,
            'fundCode': fund_code,
            'fundName': fund_name,
            'alertThreshold': threshold,
            'addedAt': datetime.now()
        })
        
        return jsonify({'message': 'Successfully added to watchlist'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/watchlist/<fund_code>', methods=['DELETE'])
@token_required
def remove_from_watchlist(current_user_id, fund_code):
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        print(f"[删除关注] 用户: {current_user_id}, 基金代码: {fund_code}")
        
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if not existing:
            print(f"[删除关注] ❌ 未找到记录: userId={current_user_id}, fundCode={fund_code}")
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        result = watchlist_collection.delete_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        print(f"[删除关注] ✅ 删除结果: deleted_count={result.deleted_count}")
        
        if result.deleted_count == 0:
            return jsonify({'error': 'Failed to delete from watchlist'}), 500
        
        return jsonify({'message': 'Successfully removed from watchlist'})
    except Exception as e:
        print(f"[删除关注] ❌ 删除失败: {str(e)}")
        return jsonify({'error': 'Failed to remove from watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['PUT'])
@token_required
def update_watchlist_threshold(current_user_id, fund_code):
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        threshold = data.get('alertThreshold')
        
        result = watchlist_collection.update_one({
            'userId': current_user_id,
            'fundCode': fund_code
        }, {
            '$set': {'alertThreshold': threshold}
        })
        
        if result.matched_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        return jsonify({'message': 'Successfully updated threshold'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

def send_verification_email(to_email, code):
    print(f"[邮件] 开始发送验证码到 {to_email}")
    
    if not RESEND_API_KEY:
        print("[邮件] ❌ RESEND_API_KEY 未配置")
        print(f"[邮件] 验证码: {code}")
        return False, f"邮件服务未配置，验证码: {code}"
    
    try:
        print("[邮件] 正在通过 Resend API 发送...")
        
        params = {
            "from": "FundTracking <no-reply@fundtracking.online>",
            "to": [to_email],
            "subject": "基金追踪系统 - 邮箱验证码",
            "html": f'''
            <html>
            <body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
            <h2 style="color: #333;">基金追踪系统</h2>
            <p>您好！</p>
            <p>您的邮箱验证码是：</p>
            <div style="background-color: #f5f5f5; padding: 20px; text-align: center; margin: 20px 0;">
                <span style="font-size: 32px; font-weight: bold; color: #4CAF50;">{code}</span>
            </div>
            <p>验证码有效期为 <strong>10分钟</strong>，请尽快完成验证。</p>
            <p style="color: #999; font-size: 12px;">如果您没有注册账号，请忽略此邮件。</p>
            </body>
            </html>
            '''
        }
        
        result = resend.Emails.send(params)
        print(f"[邮件] ✅ 发送成功: {result}")
        return True, "发送成功"
        
    except Exception as e:
        print(f"[邮件] ❌ 发送失败: {str(e)}")
        return False, f"邮件发送失败，验证码: {code}"

@app.route('/api/auth/register', methods=['POST'])
def register():
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        existing_user = users_collection.find_one({'email': email})
        if existing_user:
            return jsonify({'error': 'User already exists'}), 409
        
        verification_code = str(random.randint(100000, 999999))
        expires_at = datetime.now() + timedelta(minutes=10)
        
        pending_user = {
            'email': email,
            'password': password,
            'verification_code': verification_code,
            'verification_code_expires': expires_at,
            'createdAt': datetime.now()
        }
        
        users_collection.insert_one(pending_user)
        
        threading.Thread(target=send_verification_email, args=(email, verification_code)).start()
        
        return jsonify({'status': 'success', 'message': 'Verification code sent'}), 200
        
    except Exception as e:
        print(f"[注册] ❌ 注册失败: {str(e)}")
        return jsonify({'error': 'Registration failed'}), 500

@app.route('/api/auth/verify', methods=['POST'])
def verify_email():
    try:
        data = request.get_json()
        email = data.get('email')
        code = data.get('code')
        password = data.get('password')
        
        if not email or not code or not password:
            return jsonify({'error': 'Email, code, and password are required'}), 400
        
        pending_user = users_collection.find_one({
            'email': email,
            'verification_code': code
        })
        
        if not pending_user:
            return jsonify({'error': 'Invalid verification code'}), 400
        
        if pending_user['verification_code_expires'] < datetime.now():
            return jsonify({'error': 'Verification code expired'}), 400
        
        new_user = {
            'email': email,
            'password': password,
            'is_verified': True,
            'createdAt': datetime.now()
        }
        
        users_collection.insert_one(new_user)
        users_collection.delete_one({'email': email})
        
        return jsonify({'status': 'success', 'message': 'Email verified successfully'}), 200
        
    except Exception as e:
        print(f"[验证] ❌ 验证失败: {str(e)}")
        return jsonify({'error': 'Verification failed'}), 500

@app.route('/api/auth/login', methods=['POST'])
def login():
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        user = users_collection.find_one({'email': email})
        
        if not user:
            return jsonify({'error': 'Invalid email or password'}), 401
        
        payload = {
            'userId': str(user['_id']),
            'email': email,
            'exp': datetime.now(datetime.UTC) + timedelta(hours=24)
        }
        
        token = jwt.encode(payload, JWT_SECRET, algorithm='HS256')
        
        return jsonify({
            'status': 'success',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"[登录] ❌ 登录失败: {str(e)}")
        return jsonify({'error': 'Login failed'}), 500

@app.route('/api/auth/resend', methods=['POST'])
def resend_verification():
    try:
        data = request.get_json()
        email = data.get('email')
        
        if not email:
            return jsonify({'error': 'Email is required'}), 400
        
        pending_user = users_collection.find_one({'email': email})
        
        if not pending_user:
            return jsonify({'error': 'User not found'}), 404
        
        new_code = str(random.randint(100000, 999999))
        new_expires_at = datetime.now() + timedelta(minutes=10)
        
        users_collection.update_one(
            {'email': email},
            {'$set': {
                'verification_code': new_code,
                'verification_code_expires': new_expires_at
            }}
        )
        
        threading.Thread(target=send_verification_email, args=(email, new_code)).start()
        
        return jsonify({'status': 'success', 'message': 'New verification code sent'}), 200
        
    except Exception as e:
        print(f"[重发] ❌ 重发失败: {str(e)}")
        return jsonify({'error': 'Resend failed'}), 500

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
